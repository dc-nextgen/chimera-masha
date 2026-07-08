// Stepper — kılavuz sihirbazın numaralı, geri-dönülebilir adım çubuğu (tasarım ②-⑥).
// Tamamlanan adım ✓ + tıklanabilir (geri dön); aktif adım vurgulu; ileri adımlar pasif.
import { CheckIcon } from 'lucide-react'

export type StepDef = { key: string; title: string }

export function Stepper({
  steps,
  active,
  furthest,
  onGo,
}: {
  steps: StepDef[]
  active: number // 0-tabanlı aktif adım
  furthest: number // erişilmiş en ileri adım (buraya kadar tıklanabilir)
  onGo: (i: number) => void
}) {
  return (
    <div className="flex items-center gap-0 overflow-x-auto py-1">
      {steps.map((s, i) => {
        const done = i < active
        const isActive = i === active
        const reachable = i <= furthest
        return (
          <div key={s.key} className="flex flex-1 items-center gap-0">
            <button
              type="button"
              disabled={!reachable}
              onClick={() => reachable && onGo(i)}
              className={`flex shrink-0 items-center gap-2 whitespace-nowrap text-sm ${
                isActive ? 'font-semibold text-primary' : done ? 'text-emerald-600 dark:text-emerald-400' : 'text-muted-foreground'
              } ${reachable ? 'cursor-pointer' : 'cursor-default'}`}
            >
              <span
                className={`flex size-6 items-center justify-center rounded-full text-xs font-semibold ${
                  isActive
                    ? 'bg-primary text-primary-foreground'
                    : done
                      ? 'bg-emerald-600 text-white'
                      : 'bg-muted text-muted-foreground'
                }`}
              >
                {done ? <CheckIcon className="size-3.5" /> : i + 1}
              </span>
              <span className="hidden sm:inline">{s.title}</span>
            </button>
            {i < steps.length - 1 && <span className="bg-border mx-2 h-px min-w-3 flex-1" />}
          </div>
        )
      })}
    </div>
  )
}
