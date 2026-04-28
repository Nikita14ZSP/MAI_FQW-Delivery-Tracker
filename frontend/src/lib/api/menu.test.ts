import { describe, it, expect } from 'vitest';
import { listCategories, listItems } from './menu';

describe('menu API client', () => {
  it('listCategories returns array of categories', async () => {
    const categories = await listCategories();
    expect(categories).toHaveLength(2);
    expect(categories[0].id).toBe('cat-1');
    expect(categories[0].name).toBe('Закуски');
  });

  it('listItems() returns all items', async () => {
    const items = await listItems();
    expect(items).toHaveLength(2);
  });

  it('listItems(categoryId) filters by category', async () => {
    const items = await listItems('cat-1');
    expect(items).toHaveLength(1);
    expect(items[0].categoryId).toBe('cat-1');
  });
});
