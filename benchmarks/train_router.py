"""Train a learned escalation router on exported trace features and export
it as JSON for the Go evaluator (benchmarks/internal/sweep/learned.go).

Offline training only; nothing here runs in the gateway. Features come from
`go run ./benchmarks/cmd/features`, so Python never computes a feature.

    go run ./benchmarks/cmd/features -input benchmarks/results/collected_gsm8k.jsonl.gz,benchmarks/results/collected_mmlu.jsonl.gz -output benchmarks/results/features_train.csv
    python benchmarks/train_router.py --train benchmarks/results/features_train.csv --name full

Outputs, in --out-dir (default benchmarks/models):
    router_<name>_logistic.json, router_<name>_gbt.json   models for Go
    router_<name>_parity.csv                              rows for the Go parity test
    router_<name>_report.json                             out-of-fold AUROC, calibration, operating points
"""

import argparse
import csv
import json
import math
import os
import sys
import warnings

import numpy as np
from sklearn.ensemble import GradientBoostingClassifier
from sklearn.isotonic import IsotonicRegression
from sklearn.linear_model import LogisticRegression
from sklearn.metrics import roc_auc_score
from sklearn.model_selection import StratifiedKFold, cross_val_predict
from sklearn.preprocessing import StandardScaler

META_COLUMNS = ["source", "prompt_id", "category", "label", "heavy_correct"]
ERROR_TARGETS = [0.05, 0.08, 0.10, 0.12]


def load(path):
    with open(path, newline="", encoding="utf-8") as f:
        reader = csv.reader(f)
        header = next(reader)
        rows = list(reader)
    if header[: len(META_COLUMNS)] != META_COLUMNS:
        sys.exit(f"{path}: unexpected header prefix {header[:5]}")
    features = header[len(META_COLUMNS):]
    meta = [r[: len(META_COLUMNS)] for r in rows]
    X = np.array([[float(v) for v in r[len(META_COLUMNS):]] for r in rows], dtype=np.float64)
    y = np.array([int(r[3]) for r in rows], dtype=np.int64)
    heavy = np.array([1 if r[4] == "1" else 0 for r in rows], dtype=np.int64)
    return features, meta, X, y, heavy


def bootstrap_auroc(y, s, iterations, seed):
    rng = np.random.default_rng(seed)
    n = len(y)
    vals = []
    for _ in range(iterations):
        idx = rng.integers(0, n, n)
        yy = y[idx]
        if yy.min() == yy.max():
            continue
        vals.append(roc_auc_score(yy, s[idx]))
    vals.sort()
    point = roc_auc_score(y, s)
    return point, vals[int(0.025 * len(vals))], vals[int(0.975 * len(vals))]


def logit(p):
    p = min(max(p, 1e-12), 1 - 1e-12)
    return math.log(p / (1 - p))


def export_logistic(scaler, clf, features, prefix_tokens, name):
    return {
        "name": name,
        "kind": "logistic",
        "features": features,
        "prefix_tokens": prefix_tokens,
        "mean": scaler.mean_.tolist(),
        "scale": scaler.scale_.tolist(),
        "coef": clf.coef_[0].tolist(),
        "intercept": float(clf.intercept_[0]),
    }


def export_gbt(clf, features, prefix_tokens, name):
    prior = clf.init_.predict_proba(np.zeros((1, len(features))))[0, 1]
    trees = []
    for est in clf.estimators_[:, 0]:
        t = est.tree_
        trees.append({
            "feature": t.feature.tolist(),
            "threshold": t.threshold.tolist(),
            "left": t.children_left.tolist(),
            "right": t.children_right.tolist(),
            "value": t.value[:, 0, 0].tolist(),
        })
    return {
        "name": name,
        "kind": "gbt",
        "features": features,
        "prefix_tokens": prefix_tokens,
        "init_score": logit(prior),
        "learning_rate": float(clf.learning_rate),
        "trees": trees,
    }


def eval_exported(model, X):
    """Re-implement the Go evaluator in numpy to check the export is exact."""
    if model["kind"] == "logistic":
        scale = np.array(model["scale"])
        scale[scale == 0] = 1
        z = model["intercept"] + ((X - np.array(model["mean"])) / scale) @ np.array(model["coef"])
    else:
        z = np.full(len(X), model["init_score"])
        for tree in model["trees"]:
            feat, thr, left, right, val = (np.array(tree[k]) for k in ("feature", "threshold", "left", "right", "value"))
            for i, x in enumerate(X):
                node = 0
                while left[node] >= 0:
                    node = left[node] if x[feat[node]] <= thr[node] else right[node]
                z[i] += model["learning_rate"] * val[node]
    return 1 / (1 + np.exp(-z))


def operating_points(p_cal, y, heavy, n_total):
    """Largest acceptance set whose served error rate stays under each target.

    Records are accepted when calibrated probability <= tau. Served errors
    are wrong accepted drafts plus escalations whose heavyweight was also
    wrong. The count gets a +1 finite-sample correction, so the guarantee
    is conservative on the calibration split; the held-out run reports
    what actually happened.
    """
    order = np.argsort(p_cal)
    points = []
    for target in ERROR_TARGETS:
        best_tau = None
        best_rate = None
        total_heavy_wrong = int(((heavy == 0)).sum())
        accepted_wrong = 0
        heavy_wrong_in_accepted = 0
        for k, idx in enumerate(order):
            accepted_wrong += int(y[idx])
            heavy_wrong_in_accepted += int(heavy[idx] == 0)
            escalated_wrong = total_heavy_wrong - heavy_wrong_in_accepted
            rate = (accepted_wrong + escalated_wrong + 1) / (n_total + 1)
            if rate <= target:
                best_tau = float(p_cal[idx])
                best_rate = rate
        points.append({
            "name": f"served_error_le_{target:.2f}",
            "target": target,
            "tau": best_tau if best_tau is not None else -1.0,
            "description": (
                f"largest acceptance set with corrected served error <= {target:.0%} on calibration folds"
                + ("" if best_tau is not None else "; unattainable, always escalate")
            ),
            "calibration_rate": best_rate,
        })
    return points


def main():
    warnings.filterwarnings("ignore", message="Unknown solver options")
    ap = argparse.ArgumentParser()
    ap.add_argument("--train", required=True)
    ap.add_argument("--name", default="full")
    ap.add_argument("--prefix-tokens", type=int, default=0, help="recorded in the model so Go restricts features identically")
    ap.add_argument("--out-dir", default="benchmarks/models")
    ap.add_argument("--seed", type=int, default=42)
    ap.add_argument("--folds", type=int, default=5)
    ap.add_argument("--bootstrap", type=int, default=2000)
    args = ap.parse_args()

    features, meta, X, y, heavy = load(args.train)
    n, pos = len(y), int(y.sum())
    print(f"train: {n} records, {pos} positives ({pos / n:.1%}), {len(features)} features")
    os.makedirs(args.out_dir, exist_ok=True)

    cv = StratifiedKFold(n_splits=args.folds, shuffle=True, random_state=args.seed)
    report = {
        "train_file": args.train,
        "records": n,
        "positives": pos,
        "folds": args.folds,
        "seed": args.seed,
        "prefix_tokens": args.prefix_tokens,
        "oof_auroc": {},
        "models": {},
    }

    # Single-feature baselines on the same records, for context.
    fi = {f: i for i, f in enumerate(features)}
    for label, col, sign in (("window_peak", "window_peak", 1), ("mean_entropy", "mean_entropy", 1), ("neg_min_logprob", "min_logprob", -1)):
        pt, lo, hi = bootstrap_auroc(y, sign * X[:, fi[col]], args.bootstrap, args.seed)
        report["oof_auroc"][label] = {"auroc": pt, "ci_lower": lo, "ci_upper": hi, "note": "single feature, no fitting"}
        print(f"  {label:<24} AUROC={pt:.3f} [{lo:.3f}, {hi:.3f}]")

    def fit_logistic():
        scaler = StandardScaler().fit(X)
        clf = LogisticRegression(C=1.0, max_iter=5000).fit(scaler.transform(X), y)
        return scaler, clf

    def fit_gbt():
        return GradientBoostingClassifier(
            n_estimators=150, max_depth=2, learning_rate=0.05, subsample=0.8, random_state=args.seed
        ).fit(X, y)

    specs = [
        ("logistic", lambda: fit_logistic()),
        ("gbt", lambda: fit_gbt()),
    ]

    parity_rows = []
    for kind, fitter in specs:
        if kind == "logistic":
            oof = np.zeros(n)
            for tr, te in cv.split(X, y):
                sc = StandardScaler().fit(X[tr])
                clf = LogisticRegression(C=1.0, max_iter=5000).fit(sc.transform(X[tr]), y[tr])
                oof[te] = clf.predict_proba(sc.transform(X[te]))[:, 1]
            scaler, clf = fitter()
            model = export_logistic(scaler, clf, features, args.prefix_tokens, f"{args.name}_logistic")
            full_pred = clf.predict_proba(scaler.transform(X))[:, 1]
        else:
            oof = cross_val_predict(
                GradientBoostingClassifier(n_estimators=150, max_depth=2, learning_rate=0.05, subsample=0.8, random_state=args.seed),
                X, y, cv=cv, method="predict_proba",
            )[:, 1]
            clf = fitter()
            model = export_gbt(clf, features, args.prefix_tokens, f"{args.name}_gbt")
            full_pred = clf.predict_proba(X)[:, 1]

        exported = eval_exported(model, X)
        max_diff = float(np.max(np.abs(exported - full_pred)))
        if max_diff > 1e-8:
            sys.exit(f"{kind}: exported model disagrees with sklearn by {max_diff}")

        pt, lo, hi = bootstrap_auroc(y, oof, args.bootstrap, args.seed)
        report["oof_auroc"][kind] = {"auroc": pt, "ci_lower": lo, "ci_upper": hi, "note": f"{args.folds}-fold out-of-fold"}
        print(f"  {kind:<24} AUROC={pt:.3f} [{lo:.3f}, {hi:.3f}]  (out-of-fold)")

        iso = IsotonicRegression(out_of_bounds="clip").fit(oof, y)
        model["calibration"] = {"x": iso.X_thresholds_.tolist(), "y": iso.y_thresholds_.tolist()}
        p_cal = iso.predict(oof)
        points = operating_points(p_cal, y, heavy, n)
        model["thresholds"] = [{k: v for k, v in p.items() if k != "calibration_rate"} for p in points]
        model["train"] = {
            "file": args.train,
            "records": n,
            "positives": pos,
            "oof_auroc": pt,
            "export_max_abs_diff": max_diff,
        }
        if kind == "logistic":
            model["train"]["coef_by_feature"] = dict(zip(features, model["coef"]))
        else:
            model["train"]["feature_importance"] = dict(zip(features, clf.feature_importances_.tolist()))

        out = os.path.join(args.out_dir, f"router_{args.name}_{kind}.json")
        with open(out, "w", encoding="utf-8") as f:
            json.dump(model, f, indent=1)
        report["models"][kind] = {
            "path": out,
            "operating_points": points,
            "calibration_breakpoints": len(model["calibration"]["x"]),
        }
        print(f"  wrote {out}: {len(model['calibration']['x'])} isotonic breakpoints; operating points "
              + ", ".join(f"{p['target']:.0%}->tau {p['tau']:.3f}" for p in points))

        cal_full = iso.predict(exported)
        step = max(n // 25, 1)
        for i in range(0, n, step):
            parity_rows.append([model["name"], meta[i][1]] + [repr(float(v)) for v in X[i]] + [repr(float(exported[i])), repr(float(cal_full[i]))])

    parity_path = os.path.join(args.out_dir, f"router_{args.name}_parity.csv")
    with open(parity_path, "w", newline="", encoding="utf-8") as f:
        w = csv.writer(f)
        w.writerow(["model", "prompt_id"] + features + ["raw_probability", "calibrated_probability"])
        w.writerows(parity_rows)
    report_path = os.path.join(args.out_dir, f"router_{args.name}_report.json")
    with open(report_path, "w", encoding="utf-8") as f:
        json.dump(report, f, indent=1)
    print(f"wrote {parity_path} ({len(parity_rows)} rows) and {report_path}")


if __name__ == "__main__":
    main()
