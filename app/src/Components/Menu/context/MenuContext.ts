import { createContext } from 'react';
import type { MenuItem } from '../type/type';

export interface MenuContextProps {
  menuItems: MenuItem[];
  addMenuItem: (item: MenuItem) => void;
  setMenuItems: (items: MenuItem[]) => void;
}

export const MenuContext = createContext<MenuContextProps | undefined>(
  undefined,
);
