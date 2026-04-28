import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs';

export type StatusFilter = 'active' | 'completed' | 'all';

export interface StatusTabsProps {
  value: StatusFilter;
  onChange: (next: StatusFilter) => void;
}

export function StatusTabs({ value, onChange }: StatusTabsProps) {
  return (
    <Tabs
      value={value}
      onValueChange={(next) => onChange(next as StatusFilter)}
      className="mb-4"
    >
      <TabsList className="bg-gray-100 p-1 rounded-lg">
        <TabsTrigger value="active">Активные</TabsTrigger>
        <TabsTrigger value="completed">Завершённые</TabsTrigger>
        <TabsTrigger value="all">Все</TabsTrigger>
      </TabsList>
    </Tabs>
  );
}
