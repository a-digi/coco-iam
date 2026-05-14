import { useState } from 'react';
import type { ReactNode } from 'react';
import type { MenuItem } from './type/type';
import {MenuContext} from './context/MenuContext.ts';

export const MenuProvider = ({ children }: { children: ReactNode }) => {
  const [menuItems, setMenuItems] = useState<MenuItem[]>([]);

  const addMenuItem = (item: MenuItem) => {
    setMenuItems(prev => [...prev, item]);
  };

  return (
    <MenuContext.Provider value={{ menuItems, addMenuItem, setMenuItems }}>
      {children}
    </MenuContext.Provider>
  );
};

