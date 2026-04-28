import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs';
import type { MenuCategory } from '@/types/order';

export interface CategoryTabsProps {
  categories: MenuCategory[];
  activeId: string;
  onChange: (id: string) => void;
}

export function CategoryTabs({
  categories,
  activeId,
  onChange,
}: CategoryTabsProps) {
  if (!categories.length) return null;
  return (
    <Tabs value={activeId} onValueChange={onChange} className="w-full">
      <TabsList className="sticky top-0 z-10 flex h-auto gap-1 overflow-x-auto whitespace-nowrap bg-white pb-2 border-b border-gray-100">
        {categories.map((c) => (
          <TabsTrigger key={c.id} value={c.id} className="px-3 py-2 text-sm">
            {c.name}
          </TabsTrigger>
        ))}
      </TabsList>
    </Tabs>
  );
}
