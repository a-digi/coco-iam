import React, { useState } from 'react';
import { Link } from 'react-router-dom';
import { useAuth } from '../../../../Components/Auth/Guard/useAuth';
import { useAdminProfile } from '../../../../Components/Auth/Profile/useAdminProfile';
import { profileDisplayName } from '../../../../Components/Auth/Profile/profileDisplayName';
import { absolutePublicURL } from '../../../../api/client';

const UserMenu: React.FC = () => {
    const [isOpen, setIsOpen] = useState(false);
    const { logout } = useAuth();
    const { profile } = useAdminProfile();
    const displayName = profileDisplayName(profile);
    // avatar_url from the server is a relative /p/... path. Turn
    // it into an absolute URL pointing at the API origin so the
    // <img src> resolves regardless of where the admin UI is
    // hosted (dev proxy, static deployment, etc.).
    const avatarURL = profile?.avatar_url ? absolutePublicURL(profile.avatar_url) : '';

    return (
        <div className="relative inline-block text-left">
            <div>
                <button
                    type="button"
                    className="flex items-center gap-2 px-2 py-1.5 rounded-full hover:bg-gray-200 dark:hover:bg-surface-700 focus:outline-none transition-colors"
                    onClick={() => setIsOpen(!isOpen)}
                    aria-expanded={isOpen}
                    aria-haspopup="true"
                >
                    {avatarURL ? (
                        <img
                            src={avatarURL}
                            alt=""
                            className="w-7 h-7 rounded-full object-cover flex-shrink-0 bg-gray-100 dark:bg-surface-700"
                        />
                    ) : (
                        <div className="flex items-center justify-center w-7 h-7 rounded-full bg-blue-100 dark:bg-blue-900/50 flex-shrink-0">
                            <svg className="w-4 h-4 text-blue-600 dark:text-blue-300" fill="currentColor" viewBox="0 0 20 20">
                                <path fillRule="evenodd" d="M10 9a3 3 0 100-6 3 3 0 000 6zm-7 9a7 7 0 1114 0H3z" clipRule="evenodd" />
                            </svg>
                        </div>
                    )}
                    <span className="text-sm font-medium text-gray-700 dark:text-gray-200 pr-1 max-w-[160px] truncate">
                        {displayName}
                    </span>
                    <svg className={`w-4 h-4 text-gray-400 transition-transform ${isOpen ? 'rotate-180' : ''}`} fill="none" viewBox="0 0 24 24" stroke="currentColor">
                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
                    </svg>
                </button>
            </div>

            {isOpen && (
                <>
                    {/* Invisible backdrop to catch clicks outside */}
                    <div className="fixed inset-0 z-40" onClick={() => setIsOpen(false)} aria-hidden="true" />
                    <div
                        className="origin-top-right absolute right-0 mt-2 w-56 rounded-xl shadow-xl bg-white dark:bg-surface-900 focus:outline-none z-50 overflow-hidden border border-gray-100 dark:border-gray-700 animate-fade-in"
                        role="menu"
                        aria-orientation="vertical"
                        tabIndex={-1}
                    >
                        <div className="px-4 py-3 border-b border-gray-100 dark:border-gray-700">
                            <p className="text-xs text-gray-500 dark:text-gray-400 font-medium uppercase tracking-wider">Signed in as</p>
                            <p className="text-sm font-semibold text-gray-900 dark:text-white truncate mt-0.5">
                                {displayName}
                            </p>
                            {profile?.email && (
                                <p className="text-xs text-gray-500 dark:text-gray-400 truncate">
                                    {profile.email}
                                </p>
                            )}
                        </div>
                        <div className="py-1.5" role="none">
                            <Link
                                to="/profile"
                                className="flex items-center px-4 py-2 text-sm text-gray-700 hover:bg-gray-50 dark:text-gray-200 dark:hover:bg-surface-800 transition-colors"
                                role="menuitem"
                                onClick={() => setIsOpen(false)}
                            >
                                <svg className="mr-3 w-4 h-4 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z" /></svg>
                                Profile
                            </Link>
                            <Link
                                to="/account/change-password"
                                className="flex items-center px-4 py-2 text-sm text-gray-700 hover:bg-gray-50 dark:text-gray-200 dark:hover:bg-surface-800 transition-colors"
                                role="menuitem"
                                onClick={() => setIsOpen(false)}
                            >
                                <svg className="mr-3 w-4 h-4 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 7a2 2 0 012 2m4 0a6 6 0 01-7.743 5.743L11 17H9v2H7v2H4a1 1 0 01-1-1v-2.586a1 1 0 01.293-.707l5.964-5.964A6 6 0 1121 9z" /></svg>
                                Change password
                            </Link>
                            <div className="border-t border-gray-100 dark:border-gray-700 my-1.5"></div>
                            <button
                                onClick={() => {
                                    setIsOpen(false);
                                    logout();
                                }}
                                className="flex w-full items-center px-4 py-2 text-sm text-red-600 hover:bg-red-50 dark:text-red-400 dark:hover:bg-red-900/10 transition-colors text-left"
                                role="menuitem"
                            >
                                <svg className="mr-3 w-4 h-4 text-red-500/70 dark:text-red-400/70" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M17 16l4-4m0 0l-4-4m4 4H7m6 4v1a3 3 0 01-3 3H6a3 3 0 01-3-3V7a3 3 0 013-3h4a3 3 0 013 3v1" /></svg>
                                Logout
                            </button>
                        </div>
                    </div>
                </>
            )}
        </div>
    );
};

export default UserMenu;
