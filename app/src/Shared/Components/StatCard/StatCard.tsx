import React from 'react';

export type StatCardColor = 'blue' | 'teal' | 'violet' | 'amber';

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
};

export const StatCard: React.FC<StatCardProps> = ({ label, value, color }) => {
    const { gradient, ring } = CARD_STYLES[color];
    return (
        <div
            className={`${gradient} rounded-2xl p-5 flex flex-col justify-between shadow-lg relative overflow-hidden min-h-[110px]`}
        >
            <div className={`absolute -top-5 -right-5 w-24 h-24 rounded-full ${ring} opacity-20`} />
            <div className={`absolute -bottom-6 -left-3 w-16 h-16 rounded-full ${ring} opacity-10`} />
            <span className="text-white/70 text-xs font-semibold uppercase tracking-widest leading-tight z-10">
                {label}
            </span>
            <span className="text-white text-4xl font-bold mt-2 z-10">{value}</span>
        </div>
    );
};

export default StatCard;
