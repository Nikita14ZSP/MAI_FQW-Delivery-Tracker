import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group';
import { Label } from '@/components/ui/label';
import { cn } from '@/lib/utils';

interface Option {
  value: 'card_on_delivery' | 'cash' | 'online';
  title: string;
  hint: string;
}

const OPTIONS: Option[] = [
  {
    value: 'card_on_delivery',
    title: 'Картой курьеру',
    hint: 'Терминал у курьера',
  },
  { value: 'cash', title: 'Наличными курьеру', hint: 'Подготовьте сдачу' },
  { value: 'online', title: 'Онлайн оплата', hint: 'Тестовый режим' },
];

export interface PaymentMethodRadioProps {
  value: Option['value'];
  onChange: (v: Option['value']) => void;
}

export function PaymentMethodRadio({
  value,
  onChange,
}: PaymentMethodRadioProps) {
  return (
    <RadioGroup
      value={value}
      onValueChange={(v) => onChange(v as Option['value'])}
      className="flex flex-col gap-2"
    >
      {OPTIONS.map((o) => (
        <Label
          key={o.value}
          htmlFor={`pay-${o.value}`}
          className={cn(
            'flex items-center gap-3 p-3 rounded-lg border cursor-pointer',
            value === o.value
              ? 'border-blue-600 bg-blue-50'
              : 'border-gray-200 bg-white hover:bg-gray-50',
          )}
        >
          <RadioGroupItem id={`pay-${o.value}`} value={o.value} />
          <div className="flex flex-col gap-0">
            <span className="text-sm font-semibold text-gray-900">
              {o.title}
            </span>
            <span className="text-xs text-gray-500">{o.hint}</span>
          </div>
        </Label>
      ))}
    </RadioGroup>
  );
}
