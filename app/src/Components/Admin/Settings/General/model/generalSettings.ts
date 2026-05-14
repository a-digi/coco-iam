export interface GeneralSettings {
    base_url: string;
    page_title: string;
    description: string;
    robots: string;
}

export interface GeneralSettingsPatch {
    base_url?: string;
    page_title?: string;
    description?: string;
    robots?: string;
}

export const EMPTY_GENERAL: GeneralSettings = {
    base_url: '',
    page_title: '',
    description: '',
    robots: '',
};

export const ROBOTS_PRESETS: { label: string; value: string }[] = [
    { label: 'Public (index, follow)', value: 'index, follow' },
    { label: 'Private (noindex, nofollow)', value: 'noindex, nofollow' },
    { label: 'Index only (index, nofollow)', value: 'index, nofollow' },
    { label: 'Follow only (noindex, follow)', value: 'noindex, follow' },
];
