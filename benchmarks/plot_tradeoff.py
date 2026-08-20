"""Plot cost-reduction vs accuracy tradeoff for the three routing methods.

Reads benchmarks/results/compare.csv (produced by cmd/compare) and writes
benchmarks/results/tradeoff.png. Standalone script, not part of the Go build.
"""

import csv
import sys
from collections import defaultdict

import matplotlib.pyplot as plt

INPUT = "benchmarks/results/compare.csv"
OUTPUT = "benchmarks/results/tradeoff.png"

COLORS = {
    "entropy_router": "#2a78d6",
    "confidence_threshold": "#eb6834",
    "always_heavyweight": "#1baf7a",
}
LABELS = {
    "entropy_router": "Entropy router",
    "confidence_threshold": "Confidence threshold",
    "always_heavyweight": "Always heavyweight",
}
MARKERS = {
    "entropy_router": "o",
    "confidence_threshold": "s",
    "always_heavyweight": "*",
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
    by_method = load_rows(INPUT)
    if not by_method:
        print(f"no rows found in {INPUT}", file=sys.stderr)
        sys.exit(1)

    fig, ax = plt.subplots(figsize=(8, 6), dpi=150)
    fig.patch.set_facecolor("#fcfcfb")
    ax.set_facecolor("#fcfcfb")

    ax.grid(True, color="#e3e2dd", linewidth=0.8, zorder=0)
    for spine in ("top", "right"):
        ax.spines[spine].set_visible(False)
    for spine in ("left", "bottom"):
        ax.spines[spine].set_color("#c3c2b7")

    for method in ("entropy_router", "confidence_threshold", "always_heavyweight"):
        rows = sorted(by_method.get(method, []), key=lambda r: r["threshold"])
        if not rows:
            continue
        xs = [r["cost_reduction"] for r in rows]
        ys = [r["overall_accuracy"] for r in rows]
        color = COLORS[method]
        marker = MARKERS[method]

        if method == "always_heavyweight":
            ax.scatter(xs, ys, s=180, marker=marker, color=color,
                       label=LABELS[method], zorder=4, edgecolors="white", linewidths=0.8)
        else:
            ax.plot(xs, ys, linewidth=2, color=color, zorder=2, alpha=0.85)
            ax.scatter(xs, ys, s=45, marker=marker, color=color,
                       label=LABELS[method], zorder=3, edgecolors="white", linewidths=0.6)
            for r in rows:
                if method == "entropy_router" and abs(r["threshold"] - 2.0) < 1e-6:
                    ax.annotate(f"T={r['threshold']:g}", (r["cost_reduction"], r["overall_accuracy"]),
                                textcoords="offset points", xytext=(8, -10), fontsize=8, color="#52514e")

    ax.set_xlabel("Cost reduction vs always-heavyweight (%)", fontsize=11, color="#0b0b0b")
    ax.set_ylabel("Overall accuracy (%)", fontsize=11, color="#0b0b0b")
    ax.set_title("Cost reduction vs accuracy tradeoff", fontsize=13, color="#0b0b0b", pad=12)
    ax.set_xlim(-5, 105)
    ax.tick_params(colors="#52514e", labelsize=9)

    legend = ax.legend(loc="lower left", frameon=False, fontsize=9)
    for text in legend.get_texts():
        text.set_color("#0b0b0b")

    fig.tight_layout()
    fig.savefig(OUTPUT, facecolor=fig.get_facecolor())
    print(f"wrote {OUTPUT}")


if __name__ == "__main__":
    main()
