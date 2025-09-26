/**
 * User management and authentication for the JavaScript test project.
 *
 * This module provides:
 * - User authentication and session management
 * - User profile management
 * - Permission and role handling
 * - User preferences and settings
 * - Account validation and security
 */

import { EventEmitter } from 'events';
import { Utils } from './utils.js';

/**
 * User data model
 */
export class User {
    constructor(userData = {}) {
        this.id = userData.id || null;
        this.username = userData.username || '';
        this.email = userData.email || '';
        this.firstName = userData.firstName || '';
        this.lastName = userData.lastName || '';
        this.avatar = userData.avatar || null;
        this.roles = userData.roles || ['user'];
        this.permissions = userData.permissions || [];
        this.preferences = userData.preferences || {};
        this.isActive = userData.isActive !== undefined ? userData.isActive : true;
        this.emailVerified = userData.emailVerified || false;
        this.lastLogin = userData.lastLogin ? new Date(userData.lastLogin) : null;
        this.createdAt = userData.createdAt ? new Date(userData.createdAt) : new Date();
        this.updatedAt = userData.updatedAt ? new Date(userData.updatedAt) : new Date();
    }

    get fullName() {
        return `${this.firstName} ${this.lastName}`.trim() || this.username;
    }

    get displayName() {
        return this.fullName || this.email || this.username;
    }

    get initials() {
        if (this.firstName && this.lastName) {
            return `${this.firstName[0]}${this.lastName[0]}`.toUpperCase();
        }
        if (this.username) {
            return this.username.substring(0, 2).toUpperCase();
        }
        if (this.email) {
            return this.email.substring(0, 2).toUpperCase();
        }
        return 'U';
    }

    hasRole(role) {
        return this.roles.includes(role);
    }

    hasPermission(permission) {
        return this.permissions.includes(permission);
    }

    hasAnyRole(roles) {
        return roles.some(role => this.hasRole(role));
    }

    hasAnyPermission(permissions) {
        return permissions.some(permission => this.hasPermission(permission));
    }

    isAdmin() {
        return this.hasRole('admin') || this.hasRole('superadmin');
    }

    isModerator() {
        return this.hasRole('moderator') || this.isAdmin();
    }

    canEdit(resource) {
        return this.hasPermission(`edit:${resource}`) || this.isAdmin();
    }

    canDelete(resource) {
        return this.hasPermission(`delete:${resource}`) || this.isAdmin();
    }

    getPreference(key, defaultValue = null) {
        return this.preferences[key] !== undefined ? this.preferences[key] : defaultValue;
    }

    setPreference(key, value) {
        this.preferences[key] = value;
        this.updatedAt = new Date();
    }

    updateProfile(updates) {
        const allowedFields = [
            'firstName', 'lastName', 'avatar', 'preferences'
        ];

        for (const [key, value] of Object.entries(updates)) {
            if (allowedFields.includes(key)) {
                this[key] = value;
            }
        }

        this.updatedAt = new Date();
    }

    toJSON() {
        return {
            id: this.id,
            username: this.username,
            email: this.email,
            firstName: this.firstName,
            lastName: this.lastName,
            avatar: this.avatar,
            roles: this.roles,
            permissions: this.permissions,
            preferences: this.preferences,
            isActive: this.isActive,
            emailVerified: this.emailVerified,
            lastLogin: this.lastLogin?.toISOString(),
            createdAt: this.createdAt.toISOString(),
            updatedAt: this.updatedAt.toISOString()
        };
    }

    static fromJSON(data) {
        return new User(data);
    }
}

/**
 * Authentication session
 */
export class AuthSession {
    constructor(sessionData = {}) {
        this.token = sessionData.token || null;
        this.refreshToken = sessionData.refreshToken || null;
        this.expiresAt = sessionData.expiresAt ? new Date(sessionData.expiresAt) : null;
        this.user = sessionData.user ? new User(sessionData.user) : null;
        this.createdAt = sessionData.createdAt ? new Date(sessionData.createdAt) : new Date();
        this.lastActivity = sessionData.lastActivity ? new Date(sessionData.lastActivity) : new Date();
    }

    isValid() {
        return this.token && this.user && !this.isExpired();
    }

    isExpired() {
        return this.expiresAt && new Date() > this.expiresAt;
    }

    isExpiringSoon(minutesThreshold = 15) {
        if (!this.expiresAt) return false;
        const threshold = new Date(Date.now() + minutesThreshold * 60 * 1000);
        return this.expiresAt <= threshold;
    }

    getRemainingTime() {
        if (!this.expiresAt) return null;
        return Math.max(0, this.expiresAt.getTime() - Date.now());
    }

    updateActivity() {
        this.lastActivity = new Date();
    }

    toJSON() {
        return {
            token: this.token,
            refreshToken: this.refreshToken,
            expiresAt: this.expiresAt?.toISOString(),
            user: this.user?.toJSON(),
            createdAt: this.createdAt.toISOString(),
            lastActivity: this.lastActivity.toISOString()
        };
    }

    static fromJSON(data) {
        return new AuthSession(data);
    }
}

/**
 * User Manager for authentication and user operations
 */
export class UserManager extends EventEmitter {
    constructor(apiClient, dataStore) {
        super();

        this.apiClient = apiClient;
        this.dataStore = dataStore;
        this.currentUser = null;
        this.currentSession = null;
        this.isInitialized = false;

        this.sessionCheckInterval = null;
        this.sessionWarningShown = false;

        this.userCache = new Map();
        this.cacheTimeout = 5 * 60 * 1000;
    }

    async initialize() {
        if (this.isInitialized) return;

        try {
            await this.restoreSession();

            this.startSessionMonitoring();

            this.isInitialized = true;
            this.emit('initialized');
        } catch (error) {
            this.emit('initialization-error', error);
            throw error;
        }
    }

    async restoreSession(sessionData = null) {
        try {
            let session = null;

            if (sessionData) {
                session = AuthSession.fromJSON(sessionData);
            } else {
                const storedSession = await this.dataStore.get('user-session');
                if (storedSession) {
                    session = AuthSession.fromJSON(storedSession);
                }
            }

            if (session && session.isValid()) {
                const isValidRemotely = await this.verifySessionWithServer(session);

                if (isValidRemotely) {
                    this.currentSession = session;
                    this.currentUser = session.user;
                    this.setupAuthHeaders();

                    this.emit('session-restored', {
                        user: this.currentUser,
                        session: this.currentSession
                    });

                    return true;
                } else {
                    await this.clearSession();
                }
            }

            return false;
        } catch (error) {
            this.emit('session-restore-error', error);
            await this.clearSession();
            return false;
        }
    }

    async verifySessionWithServer(session) {
        try {
            const response = await this.apiClient.get('/auth/verify', {
                headers: {
                    Authorization: `Bearer ${session.token}`
                }
            });

            return response.ok && response.data?.valid;
        } catch (error) {
            return false;
        }
    }

    async login(credentials) {
        try {
            this.emit('login-start', credentials);

            const validationErrors = this.validateCredentials(credentials);
            if (validationErrors.length > 0) {
                throw new Error(validationErrors.join(', '));
            }

            const response = await this.apiClient.post('/auth/login', credentials);

            if (response.ok && response.data) {
                const sessionData = response.data;
                const session = new AuthSession({
                    token: sessionData.token,
                    refreshToken: sessionData.refreshToken,
                    expiresAt: sessionData.expiresAt,
                    user: sessionData.user
                });

                await this.setCurrentSession(session);

                this.emit('login-success', {
                    user: this.currentUser,
                    session: this.currentSession
                });

                return {
                    success: true,
                    user: this.currentUser,
                    session: this.currentSession
                };
            } else {
                throw new Error(response.data?.message || 'Login failed');
            }
        } catch (error) {
            this.emit('login-error', error);
            throw error;
        }
    }

    async logout() {
        try {
            this.emit('logout-start');

            if (this.currentSession?.token) {
                try {
                    await this.apiClient.post('/auth/logout', {
                        token: this.currentSession.token
                    });
                } catch (error) {
                    console.warn('Failed to notify server of logout:', error);
                }
            }

            await this.clearSession();

            this.emit('logout-success');

            return { success: true };
        } catch (error) {
            this.emit('logout-error', error);
            throw error;
        }
    }

    async register(userData) {
        try {
            this.emit('register-start', userData);

            const validationErrors = this.validateRegistrationData(userData);
            if (validationErrors.length > 0) {
                throw new Error(validationErrors.join(', '));
            }

            const response = await this.apiClient.post('/auth/register', userData);

            if (response.ok && response.data) {
                const user = new User(response.data.user);

                this.emit('register-success', { user });

                return {
                    success: true,
                    user,
                    message: response.data.message
                };
            } else {
                throw new Error(response.data?.message || 'Registration failed');
            }
        } catch (error) {
            this.emit('register-error', error);
            throw error;
        }
    }

    async refreshSession() {
        if (!this.currentSession?.refreshToken) {
            throw new Error('No refresh token available');
        }

        try {
            const response = await this.apiClient.post('/auth/refresh', {
                refreshToken: this.currentSession.refreshToken
            });

            if (response.ok && response.data) {
                const sessionData = response.data;
                const newSession = new AuthSession({
                    ...this.currentSession.toJSON(),
                    token: sessionData.token,
                    refreshToken: sessionData.refreshToken || this.currentSession.refreshToken,
                    expiresAt: sessionData.expiresAt
                });

                await this.setCurrentSession(newSession);

                this.emit('session-refreshed', {
                    session: this.currentSession
                });

                return this.currentSession;
            } else {
                throw new Error('Failed to refresh session');
            }
        } catch (error) {
            this.emit('session-refresh-error', error);
            await this.clearSession();
            throw error;
        }
    }

    async updateProfile(updates) {
        if (!this.isAuthenticated()) {
            throw new Error('User not authenticated');
        }

        try {
            this.emit('profile-update-start', updates);

            const response = await this.apiClient.put('/user/profile', updates);

            if (response.ok && response.data) {
                const updatedUser = new User(response.data.user);
                this.currentUser = updatedUser;
                this.currentSession.user = updatedUser;

                await this.saveSession();

                this.emit('profile-updated', { user: updatedUser });

                return { success: true, user: updatedUser };
            } else {
                throw new Error(response.data?.message || 'Profile update failed');
            }
        } catch (error) {
            this.emit('profile-update-error', error);
            throw error;
        }
    }

    async changePassword(currentPassword, newPassword) {
        if (!this.isAuthenticated()) {
            throw new Error('User not authenticated');
        }

        try {
            const passwordValidation = Utils.Validation.validatePasswordStrength(newPassword);
            if (!passwordValidation.isValid) {
                throw new Error('Password does not meet requirements: ' + passwordValidation.feedback.join(', '));
            }

            const response = await this.apiClient.put('/user/password', {
                currentPassword,
                newPassword
            });

            if (response.ok) {
                this.emit('password-changed');
                return { success: true };
            } else {
                throw new Error(response.data?.message || 'Password change failed');
            }
        } catch (error) {
            this.emit('password-change-error', error);
            throw error;
        }
    }

    async resetPassword(email) {
        try {
            if (!Utils.Validation.isValidEmail(email)) {
                throw new Error('Invalid email address');
            }

            const response = await this.apiClient.post('/auth/reset-password', { email });

            if (response.ok) {
                return { success: true, message: 'Password reset email sent' };
            } else {
                throw new Error(response.data?.message || 'Password reset failed');
            }
        } catch (error) {
            this.emit('password-reset-error', error);
            throw error;
        }
    }

    async getUserById(userId, useCache = true) {
        try {
            // Check cache first
            if (useCache && this.userCache.has(userId)) {
                const cachedEntry = this.userCache.get(userId);
                if (Date.now() - cachedEntry.timestamp < this.cacheTimeout) {
                    return cachedEntry.user;
                }
            }

            const response = await this.apiClient.get(`/users/${userId}`);

            if (response.ok && response.data) {
                const user = new User(response.data.user);

                this.userCache.set(userId, {
                    user,
                    timestamp: Date.now()
                });

                return user;
            } else {
                throw new Error('User not found');
            }
        } catch (error) {
            this.emit('user-fetch-error', { userId, error });
            throw error;
        }
    }

    async setCurrentSession(session) {
        this.currentSession = session;
        this.currentUser = session.user;

        await this.saveSession();
        this.setupAuthHeaders();

        session.updateActivity();
    }

    async saveSession() {
        if (this.currentSession) {
            await this.dataStore.set('user-session', this.currentSession.toJSON());
        }
    }

    async clearSession() {
        this.currentSession = null;
        this.currentUser = null;
        this.sessionWarningShown = false;

        await this.dataStore.remove('user-session');
        this.clearAuthHeaders();

        if (this.sessionCheckInterval) {
            clearInterval(this.sessionCheckInterval);
            this.sessionCheckInterval = null;
        }
    }

    setupAuthHeaders() {
        if (this.currentSession?.token) {
            this.apiClient.setAuthToken(this.currentSession.token);
        }
    }

    clearAuthHeaders() {
        this.apiClient.clearAuthToken();
    }

    startSessionMonitoring() {
        this.sessionCheckInterval = setInterval(() => {
            this.checkSessionStatus();
        }, 60 * 1000);
    }

    async checkSessionStatus() {
        if (!this.currentSession) return;

        if (this.currentSession.isExpired()) {
            this.emit('session-expired');
            await this.clearSession();
            return;
        }

        if (this.currentSession.isExpiringSoon(15) && !this.sessionWarningShown) {
            this.sessionWarningShown = true;
            this.emit('session-expiring', {
                remainingTime: this.currentSession.getRemainingTime()
            });
        }

        if (this.currentSession.isExpiringSoon(5) && this.currentSession.refreshToken) {
            try {
                await this.refreshSession();
                this.sessionWarningShown = false;
            } catch (error) {
                console.warn('Failed to refresh session:', error);
            }
        }
    }

    validateCredentials(credentials) {
        const errors = [];

        if (!credentials.email && !credentials.username) {
            errors.push('Email or username is required');
        }

        if (credentials.email && !Utils.Validation.isValidEmail(credentials.email)) {
            errors.push('Invalid email format');
        }

        if (!credentials.password) {
            errors.push('Password is required');
        }

        return errors;
    }

    validateRegistrationData(userData) {
        const errors = [];

        if (!userData.username || userData.username.length < 3) {
            errors.push('Username must be at least 3 characters');
        }

        if (!Utils.Validation.isValidEmail(userData.email)) {
            errors.push('Valid email is required');
        }

        if (!userData.firstName) {
            errors.push('First name is required');
        }

        if (!userData.lastName) {
            errors.push('Last name is required');
        }

        const passwordValidation = Utils.Validation.validatePasswordStrength(userData.password);
        if (!passwordValidation.isValid) {
            errors.push(...passwordValidation.feedback);
        }

        if (userData.password !== userData.confirmPassword) {
            errors.push('Passwords do not match');
        }

        return errors;
    }

    isAuthenticated() {
        return this.currentSession?.isValid() && this.currentUser !== null;
    }

    getCurrentUser() {
        return this.currentUser;
    }

    getCurrentSession() {
        return this.currentSession;
    }

    hasRole(role) {
        return this.currentUser?.hasRole(role) || false;
    }

    hasPermission(permission) {
        return this.currentUser?.hasPermission(permission) || false;
    }

    isAdmin() {
        return this.currentUser?.isAdmin() || false;
    }

    isModerator() {
        return this.currentUser?.isModerator() || false;
    }

    canEdit(resource) {
        return this.currentUser?.canEdit(resource) || false;
    }

    canDelete(resource) {
        return this.currentUser?.canDelete(resource) || false;
    }

    async cleanup() {
        if (this.sessionCheckInterval) {
            clearInterval(this.sessionCheckInterval);
        }

        await this.clearSession();
        this.userCache.clear();
        this.emit('cleanup');
    }
}

export function createUserManager(apiClient, dataStore) {
    return new UserManager(apiClient, dataStore);
}

export { User, AuthSession, UserManager };
export default UserManager;