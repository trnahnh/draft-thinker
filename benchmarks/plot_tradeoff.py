"""Plot cost-reduction vs accuracy tradeoff for every routing method.

Reads a comparison CSV produced by cmd/compare and writes a PNG. Swept
methods (entropy router, confidence threshold) are drawn as curves; the
single-point baselines (always drafter, always heavyweight, rate-matched
random, mean-entropy router, oracle) as markers. Standalone script, not
part of the Go build.

    python benchmarks/plot_tradeoff.py
    python benchmarks/plot_tradeoff.py benchmarks/results/compare_gsm8k.csv benchmarks/results/tradeoff_gsm8k.png "GSM8K (n=300)"
"""

import csv
import sys
from collections import defaultdict

import matplotlib.pyplot as plt

DEFAULT_INPUT = "benchmarks/results/compare.csv"
DEFAULT_OUTPUT = "benchmarks/results/tradeoff.png"

CURVES = {
    "entropy_router": ("Entropy router (windowed, swept T)", "#2a78d6", "o"),
    "confidence_threshold": ("Confidence threshold (min logprob, swept)", "#eb6834", "s"),
}
POINTS = {
    "always_heavyweight": ("Always heavyweight", "#1baf7a", "*", 200),
    "always_drafter": ("Always drafter", "#7a7a72", "D", 90),
    "random_matched_rate": ("Random, rate-matched to T=2.0", "#b03a8e", "X", 110),
    "mean_entropy_router": ("Mean-entropy router, rate-matched", "#0f8f9f", "P", 110),
    "oracle": ("Oracle (perfect labels)", "#c9a400", "^", 110),
}


def load_rows(path):
    by_method = defaultdict(list)
    with open(path, newline="", encoding="utf-8") as f:
        for row in csv.DictReader(f):
            by_method[row["method"]].append(
                {
                    "threshold": float(row["threshold"]),
                    "cost_reduction": float(row["cost_reduction"]) * 100,
                    "overall_accuracy": float(row["overall_accuracy"]) * 100,
                    "escalation_rate": float(row["escalation_rate"]) * 100,
                }
            )
    return by_method


def main():
    inp = sys.argv[1] if len(sys.argv) > 1 else DEFAULT_INPUT
    out = sys.argv[2] if len(sys.argv) > 2 else DEFAULT_OUTPUT
    subtitle = sys.argv[3] if len(sys.argv) > 3 else "Project set (n=519, LLM-as-judge)"

    by_method = load_rows(inp)
    if not by_method:
        print(f"no rows found in {inp}", file=sys.stderr)
        sys.exit(1)

    fig, ax = plt.subplots(figsize=(8.5, 6), dpi=150)
    fig.patch.set_facecolor("#fcfcfb")
    ax.set_facecolor("#fcfcfb")

    ax.grid(True, color="#e3e2dd", linewidth=0.8, zorder=0)
    for spine in ("top", "right"):
        ax.spines[spine].set_visible(False)
    for spine in ("left", "bottom"):
        ax.spines[spine].set_color("#c3c2b7")

    for method, (label, color, marker) in CURVES.items():
        rows = sorted(by_method.get(method, []), key=lambda r: r["threshold"])
        if not rows:
            continue
        xs = [r["cost_reduction"] for r in rows]
        ys = [r["overall_accuracy"] for r in rows]
        ax.plot(xs, ys, linewidth=2, color=color, zorder=2, alpha=0.85)
        ax.scatter(xs, ys, s=45, marker=marker, color=color, label=label,
                   zorder=3, edgecolors="white", linewidths=0.6)
        for r in rows:
            if method == "entropy_router" and abs(r["threshold"] - 2.0) < 1e-6:
                ax.annotate("T=2.0", (r["cost_reduction"], r["overall_accuracy"]),
                            textcoords="offset points", xytext=(8, -12), fontsize=8, color="#52514e")

    for method, (label, color, marker, size) in POINTS.items():
        rows = by_method.get(method, [])
        if not rows:
            continue
        r = rows[0]
        ax.scatter([r["cost_reduction"]], [r["overall_accuracy"]], s=size, marker=marker,
                   color=color, label=label, zorder=4, edgecolors="white", linewidths=0.8)

    ax.set_xlabel("Cost reduction vs always-heavyweight (%)", fontsize=11, color="#0b0b0b")
    ax.set_ylabel("Served accuracy (%)", fontsize=11, color="#0b0b0b")
    ax.set_title(f"Cost reduction vs accuracy: {subtitle}", fontsize=12, color="#0b0b0b", pad=12)
    ax.set_xlim(-10, 100)
    ax.tick_params(colors="#52514e", labelsize=9)

    legend = ax.legend(loc="lower left", frameon=False, fontsize=8)
    for text in legend.get_texts():
        text.set_color("#0b0b0b")

    fig.tight_layout()
    fig.savefig(out, facecolor=fig.get_facecolor())
    print(f"wrote {out}")


if __name__ == "__main__":
    main()
