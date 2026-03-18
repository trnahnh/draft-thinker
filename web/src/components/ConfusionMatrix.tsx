export default function ConfusionMatrix() {
  return (
    <div>
      <div className="grid grid-cols-[auto_1fr_1fr] grid-rows-[auto_1fr_1fr] text-center">
        <div />
        <div className="border-b border-(--color-border) px-2 pb-2 font-mono text-[10px] uppercase tracking-widest text-(--color-text-dark)">
          Accept
        </div>
        <div className="border-b border-(--color-border) px-2 pb-2 font-mono text-[10px] uppercase tracking-widest text-(--color-text-dark)">
          Escalate
        </div>

        <div className="flex items-center border-r border-(--color-border) pr-2 font-mono text-[10px] uppercase tracking-widest text-(--color-text-dark) [writing-mode:vertical-lr] rotate-180">
          Actual
        </div>
        <div className="border-r border-b border-(--color-border) bg-(--color-green-dim) p-3 sm:p-5">
          <div className="font-mono text-xl font-medium text-(--color-green) sm:text-2xl">
            478
          </div>
          <div className="font-mono text-[10px] text-(--color-text-dark)">
            TN
          </div>
        </div>
        <div className="border-b border-(--color-border) bg-(--color-amber-dim) p-3 sm:p-5">
          <div className="font-mono text-xl font-medium text-(--color-amber) sm:text-2xl">
            29
          </div>
          <div className="font-mono text-[10px] text-(--color-text-dark)">
            FP
          </div>
        </div>

        <div className="flex items-center border-r border-(--color-border) pr-2" />
        <div className="border-r border-(--color-border) bg-(--color-red-dim) p-3 sm:p-5">
          <div className="font-mono text-xl font-medium text-(--color-red) sm:text-2xl">
            9
          </div>
          <div className="font-mono text-[10px] text-(--color-text-dark)">
            FN
          </div>
        </div>
        <div className="bg-(--color-green-dim) p-3 sm:p-5">
          <div className="font-mono text-xl font-medium text-(--color-green) sm:text-2xl">
            2
          </div>
          <div className="font-mono text-[10px] text-(--color-text-dark)">
            TP
          </div>
        </div>
      </div>
      <div className="mt-2 flex justify-between font-mono text-[10px] text-(--color-text-dark)">
        <span>n=518</span>
        <span>T=2.0 bits</span>
      </div>
    </div>
  );
}
