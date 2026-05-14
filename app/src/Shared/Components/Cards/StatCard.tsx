import React from 'react';

export type StatCardColor = 'blue' | 'teal' | 'violet' | 'amber' | 'indigo' | 'rose';

export interface StatCardProps {
    label: string;
    value: number;
    color: StatCardColor;
}

const CARD_STYLES: Record<StatCardColor, { gradient: string; ring: string }> = {
    blue:   { gradient: 'bg-gradient-to-br from-blue-500 to-blue-700',     ring: 'bg-blue-400' },
    teal:   { gradient: 'bg-gradient-to-br from-teal-500 to-teal-700',     ring: 'bg-teal-400' },
    violet: { gradient: 'bg-gradient-to-br from-violet-500 to-violet-700', ring: 'bg-violet-400' },
    amber:  { gradient: 'bg-gradient-to-br from-amber-400 to-amber-600',   ring: 'bg-amber-300' },
    indigo: { gradient: 'bg-gradient-to-br from-indigo-500 to-indigo-700', ring: 'bg-indigo-400' },
    rose:   { gradient: 'bg-gradient-to-br from-rose-500 to-rose-700',     ring: 'bg-rose-400' },
};

export const StatCard: React.FC<StatCardProps> = ({ label, value, color }) => {
    const { gradient, ring } = CARD_STYLES[color];
    return (
        <div className={`${gradient} rounded-2xl p-5 flex flex-col items-center justify-center gap-2 shadow-lg relative overflow-hidden min-h-[110px] h-full`}>
            <div className={`absolute -top-5 -right-5 w-24 h-24 rounded-full ${ring} opacity-20`} />
            <div className={`absolute -bottom-6 -left-3 w-16 h-16 rounded-full ${ring} opacity-10`} />
            <span className="text-white/70 text-xs font-semibold uppercase tracking-widest leading-tight z-10 text-center">
                {label}
            </span>
            <span className="text-white text-4xl font-bold z-10 text-center">{value}</span>
        </div>
    );
};

export default StatCard;
