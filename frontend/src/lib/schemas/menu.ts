import { z } from 'zod';

export const MenuCategorySchema = z.object({
  id: z.string(),
  name: z.string(),
  sortOrder: z.number().int(),
});

export const MenuItemSchema = z.object({
  id: z.string(),
  categoryId: z.string(),
  name: z.string(),
  price: z.number().positive(),
  imageUrl: z.string(),
  available: z.boolean(),
});

export type MenuCategoryDTO = z.infer<typeof MenuCategorySchema>;
export type MenuItemDTO = z.infer<typeof MenuItemSchema>;
