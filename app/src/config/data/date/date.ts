export const DateFormatLocale = {
    EN_US: 'en-US',
    EN_GB: 'en-GB',
    EN_CA: 'en-CA',
    EN_AU: 'en-AU',
    DE_DE: 'de-DE',
    FR_FR: 'fr-FR',
    ES_ES: 'es-ES',
    IT_IT: 'it-IT',
    JA_JP: 'ja-JP',
    ZH_CN: 'zh-CN',
    RU_RU: 'ru-RU',
    PT_BR: 'pt-BR',
    NL_NL: 'nl-NL'
} as const;

export type DateFormatLocale = typeof DateFormatLocale[keyof typeof DateFormatLocale];

export const formatDateOnly = (
    dateString?: string | null | Date,
    locale: DateFormatLocale = DateFormatLocale.EN_GB
): string => {
    if (!dateString) return '-';
    try {
        const date = new Date(dateString);
        if (isNaN(date.getTime())) return '-';
        return new Intl.DateTimeFormat(locale, {
            year: 'numeric',
            month: '2-digit',
            day: '2-digit',
        }).format(date);
    } catch {
        return '-';
    }
};

export const formatDate = (
    dateString?: string | null | Date,
    locale: DateFormatLocale = DateFormatLocale.EN_US
): string => {
    if (!dateString) return '-';

    try {
        const date = new Date(dateString);
        if (isNaN(date.getTime())) return '-';

        return new Intl.DateTimeFormat(locale, {
            year: 'numeric',
            month: '2-digit',
            day: '2-digit',
            hour: '2-digit',
            minute: '2-digit',
            second: '2-digit',
            timeZoneName: 'short'
        }).format(date);
    } catch (error) {
        console.error('Error formatting date:', error);
        return '-';
    }
};
