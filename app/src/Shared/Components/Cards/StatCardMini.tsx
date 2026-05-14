import React from 'react';
import type { StatCardColor } from './StatCard';

export interface StatCardMiniProps {
    label: string;
    value: number;
    color: StatCardColor;
}

const MINI_STYLES: Record<StatCardColor, { gradient: string; ring: string }> = {
    blue:   { gradient: 'bg-gradient-to-br from-blue-500 to-blue-700',     ring: 'bg-blue-400' },
    teal:   { gradient: 'bg-gradient-to-br from-teal-500 to-teal-700',     ring: 'bg-teal-400' },
    violet: { gradient: 'bg-gradient-to-br from-violet-500 to-violet-700', ring: 'bg-violet-400' },
    amber:  { gradient: 'bg-gradient-to-br from-amber-400 to-amber-600',   ring: 'bg-amber-300' },
    indigo: { gradient: 'bg-gradient-to-br from-indigo-500 to-indigo-700', ring: 'bg-indigo-400' },
    rose:   { gradient: 'bg-gradient-to-br from-rose-500 to-rose-700',     ring: 'bg-rose-400' },
};

export const StatCardMini: React.FC<StatCardMiniProps> = ({ label, value, color }) => {
    const { gradient, ring } = MINI_STYLES[color];
    return (
        <div className={`${gradient} rounded-xl px-4 py-4 flex flex-col items-center justify-center text-center shadow-md relative overflow-hidden min-h-[80px]`}>
            <div className={`absolute -top-3 -right-3 w-14 h-14 rounded-full ${ring} opacity-20`} />
            <span className="text-white text-2xl font-bold z-10">{value}</span>
            <span className="text-white/70 text-[0.625rem] font-semibold uppercase tracking-widest mt-0.5 z-10 text-center leading-tight">
                {label}
            </span>
        </div>
    );
};

export default StatCardMini;
