import { api } from '@/lib/api';
import type { MenuCategory, MenuItem } from '@/types/order';

export async function listCategories(): Promise<MenuCategory[]> {
  const { data } = await api.get<{ categories: MenuCategory[] }>(
    '/menu/categories',
  );
  return data.categories ?? [];
}

export async function listItems(categoryId?: string): Promise<MenuItem[]> {
  const params = categoryId ? { category_id: categoryId } : undefined;
  const { data } = await api.get<{ items: MenuItem[] }>('/menu/items', {
    params,
  });
  return data.items ?? [];
}
