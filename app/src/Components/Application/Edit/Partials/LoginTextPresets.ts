// A LoginTextPreset is a ready-made side-panel copy block — title +
// ordered content entries — that the admin can drop into a text
// column with one click. The shape mirrors what the RichTextEditor
// stores: each string is rendered verbatim as HTML on the public
// login page, so values here must be HTML-safe (author-controlled
// static strings, not user input).
export interface LoginTextPreset {
    id: string;
    label: string;
    description: string;
    title: string;
    contents: string[];
}

export const LOGIN_TEXT_PRESETS: LoginTextPreset[] = [
    {
        id: 'welcome',
        label: 'Welcome back',
        description: 'Friendly greeting with a short guidance line.',
        title: 'Welcome back',
        contents: [
            'Sign in to continue working with your team. Your data is secure, backed up, and ready whenever you are.',
            'New here? Reach out to your administrator to get an account.',
        ],
    },
    {
        id: 'features',
        label: 'Feature highlights',
        description: 'Three short bullets summarising the product.',
        title: 'Everything in one place',
        contents: [
            '<strong>Single sign-on</strong> across all your applications.',
            '<strong>Role-based access</strong> that respects boundaries.',
            '<strong>Audit logs</strong> that let you sleep at night.',
        ],
    },
    {
        id: 'why-us',
        label: 'Built for teams',
        description: 'Short pitch with a trust line.',
        title: 'Built for teams that move fast',
        contents: [
            'Stop wiring the same auth into every new tool. One console, every application protected, zero drift.',
            'Trusted by engineering teams shipping at scale.',
        ],
    },
    {
        id: 'testimonial',
        label: 'Testimonial',
        description: 'Quote with attribution and a short CTA line.',
        title: '&ldquo;Finally, auth we don&rsquo;t hate.&rdquo;',
        contents: [
            '&mdash; Alex Chen, Lead Platform Engineer',
            'Join the teams who&rsquo;ve cut their identity stack after switching.',
        ],
    },
];
