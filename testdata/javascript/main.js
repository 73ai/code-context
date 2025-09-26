/**
 * Main application entry point for the JavaScript test project.
 *
 * This module demonstrates:
 * - ES6+ features (classes, async/await, destructuring)
 * - Module imports/exports
 * - Event handling and DOM manipulation
 * - API integration
 * - Error handling and logging
 */

import { EventEmitter } from 'events';
import { ApiClient } from './api-client.js';
import { UserManager } from './user-manager.js';
import { DataStore } from './data-store.js';
import { Utils } from './utils.js';
import { Logger } from './logger.js';

// Application configuration
const APP_CONFIG = {
    apiBaseUrl: process.env.API_BASE_URL || 'https://api.example.com',
    apiKey: process.env.API_KEY || 'demo-key',
    debug: process.env.NODE_ENV === 'development',
    maxRetries: 3,
    timeout: 30000,
    version: '1.0.0'
};

/**
 * Main application class that orchestrates all components
 */
class Application extends EventEmitter {
    constructor(config = APP_CONFIG) {
        super();

        this.config = { ...APP_CONFIG, ...config };
        this.logger = new Logger(this.config.debug);
        this.dataStore = new DataStore();
        this.apiClient = new ApiClient(this.config.apiBaseUrl, this.config.apiKey);
        this.userManager = new UserManager(this.apiClient, this.dataStore);

        this.isInitialized = false;
        this.isRunning = false;

        this.handleError = this.handleError.bind(this);
        this.handleUserAction = this.handleUserAction.bind(this);
        this.handleDataUpdate = this.handleDataUpdate.bind(this);

        this.logger.info('Application instance created', { version: this.config.version });
    }

    /**
     * Initialize the application
     */
    async initialize() {
        if (this.isInitialized) {
            this.logger.warn('Application already initialized');
            return;
        }

        try {
            this.logger.info('Initializing application...');

            await this.dataStore.initialize();
            await this.apiClient.initialize();
            await this.userManager.initialize();

            this.setupEventListeners();

            await this.loadInitialData();

            this.isInitialized = true;
            this.emit('initialized');

            this.logger.info('Application initialized successfully');
        } catch (error) {
            this.logger.error('Failed to initialize application', error);
            throw new Error(`Application initialization failed: ${error.message}`);
        }
    }

    /**
     * Start the application
     */
    async start() {
        if (!this.isInitialized) {
            await this.initialize();
        }

        if (this.isRunning) {
            this.logger.warn('Application already running');
            return;
        }

        try {
            this.logger.info('Starting application...');

            this.startHeartbeat();
            this.startPeriodicTasks();

            if (typeof window !== 'undefined') {
                this.setupDOMEventListeners();
            }

            this.isRunning = true;
            this.emit('started');

            this.logger.info('Application started successfully');
        } catch (error) {
            this.logger.error('Failed to start application', error);
            throw new Error(`Application startup failed: ${error.message}`);
        }
    }

    /**
     * Stop the application
     */
    async stop() {
        if (!this.isRunning) {
            this.logger.warn('Application not running');
            return;
        }

        try {
            this.logger.info('Stopping application...');

            this.stopHeartbeat();
            this.stopPeriodicTasks();

            await this.userManager.cleanup();
            await this.apiClient.cleanup();
            await this.dataStore.cleanup();

            this.removeAllListeners();

            this.isRunning = false;
            this.emit('stopped');

            this.logger.info('Application stopped successfully');
        } catch (error) {
            this.logger.error('Error stopping application', error);
            throw new Error(`Application stop failed: ${error.message}`);
        }
    }

    /**
     * Setup internal event listeners
     */
    setupEventListeners() {
        this.userManager.on('user-login', this.handleUserAction);
        this.userManager.on('user-logout', this.handleUserAction);
        this.userManager.on('user-profile-updated', this.handleUserAction);

        this.dataStore.on('data-changed', this.handleDataUpdate);
        this.dataStore.on('storage-error', this.handleError);

        this.apiClient.on('request-start', (event) => {
            this.logger.debug('API request started', event);
        });
        this.apiClient.on('request-complete', (event) => {
            this.logger.debug('API request completed', event);
        });
        this.apiClient.on('request-error', this.handleError);

        this.on('error', this.handleError);

        this.logger.debug('Event listeners setup complete');
    }

    /**
     * Setup DOM event listeners for browser environment
     */
    setupDOMEventListeners() {
        if (typeof window === 'undefined') return;

        document.addEventListener('visibilitychange', () => {
            if (document.hidden) {
                this.logger.debug('Page hidden, pausing non-essential tasks');
                this.emit('app-paused');
            } else {
                this.logger.debug('Page visible, resuming tasks');
                this.emit('app-resumed');
            }
        });

        window.addEventListener('beforeunload', (event) => {
            this.logger.info('Page unloading, cleaning up...');
            this.stop().catch(error => {
                this.logger.error('Error during cleanup', error);
            });
        });

        window.addEventListener('online', () => {
            this.logger.info('Network connection restored');
            this.emit('network-online');
        });

        window.addEventListener('offline', () => {
            this.logger.warn('Network connection lost');
            this.emit('network-offline');
        });

        this.logger.debug('DOM event listeners setup complete');
    }

    /**
     * Load initial data required by the application
     */
    async loadInitialData() {
        try {
            this.logger.info('Loading initial data...');

            const remoteConfig = await this.apiClient.get('/config');
            if (remoteConfig) {
                this.config = { ...this.config, ...remoteConfig };
                this.logger.debug('Remote configuration loaded', remoteConfig);
            }

            const sessionData = await this.dataStore.getItem('user-session');
            if (sessionData) {
                await this.userManager.restoreSession(sessionData);
                this.logger.debug('User session restored');
            }

            const cachedData = await this.dataStore.getItem('app-cache');
            if (cachedData) {
                this.emit('cached-data-loaded', cachedData);
                this.logger.debug('Cached data loaded');
            }

            this.logger.info('Initial data loading complete');
        } catch (error) {
            this.logger.warn('Some initial data failed to load', error);
        }
    }

    /**
     * Start heartbeat for keep-alive functionality
     */
    startHeartbeat() {
        if (this.heartbeatInterval) {
            clearInterval(this.heartbeatInterval);
        }

        this.heartbeatInterval = setInterval(async () => {
            try {
                await this.apiClient.post('/heartbeat', {
                    timestamp: new Date().toISOString(),
                    version: this.config.version
                });
                this.emit('heartbeat');
            } catch (error) {
                this.logger.warn('Heartbeat failed', error);
            }
        }, 60000);

        this.logger.debug('Heartbeat started');
    }

    /**
     * Stop heartbeat
     */
    stopHeartbeat() {
        if (this.heartbeatInterval) {
            clearInterval(this.heartbeatInterval);
            this.heartbeatInterval = null;
            this.logger.debug('Heartbeat stopped');
        }
    }

    /**
     * Start periodic background tasks
     */
    startPeriodicTasks() {
        this.cacheCleanupInterval = setInterval(() => {
            this.dataStore.cleanupExpired();
        }, 300000);

        this.dataSyncInterval = setInterval(async () => {
            try {
                await this.syncData();
            } catch (error) {
                this.logger.error('Data sync failed', error);
            }
        }, 600000);

        this.logger.debug('Periodic tasks started');
    }

    /**
     * Stop periodic tasks
     */
    stopPeriodicTasks() {
        if (this.cacheCleanupInterval) {
            clearInterval(this.cacheCleanupInterval);
            this.cacheCleanupInterval = null;
        }

        if (this.dataSyncInterval) {
            clearInterval(this.dataSyncInterval);
            this.dataSyncInterval = null;
        }

        this.logger.debug('Periodic tasks stopped');
    }

    /**
     * Sync local data with remote server
     */
    async syncData() {
        try {
            this.logger.debug('Starting data synchronization...');

            const localData = await this.dataStore.getAllData();
            const serverData = await this.apiClient.post('/sync', localData);

            if (serverData && serverData.updates) {
                await this.dataStore.batchUpdate(serverData.updates);
                this.emit('data-synced', serverData.updates);
                this.logger.info('Data synchronization completed');
            }
        } catch (error) {
            this.logger.error('Data synchronization failed', error);
            this.emit('sync-failed', error);
        }
    }

    /**
     * Handle user action events
     */
    handleUserAction(event) {
        this.logger.info('User action:', event);

        switch (event.type) {
            case 'user-login':
                this.emit('user-authenticated', event.user);
                break;
            case 'user-logout':
                this.dataStore.removeItem('user-session');
                this.emit('user-session-ended');
                break;
            case 'user-profile-updated':
                this.emit('user-data-changed', event.user);
                break;
            default:
                this.logger.debug('Unknown user action type:', event.type);
        }
    }

    /**
     * Handle data update events
     */
    handleDataUpdate(event) {
        this.logger.debug('Data updated:', event);
        this.emit('app-data-changed', event);
    }

    /**
     * Central error handler
     */
    handleError(error) {
        this.logger.error('Application error:', error);

        if (error.name === 'NetworkError') {
            this.emit('network-error', error);
        } else if (error.name === 'ValidationError') {
            this.emit('validation-error', error);
        } else if (error.name === 'AuthenticationError') {
            this.emit('auth-error', error);
        } else {
            this.emit('generic-error', error);
        }

        if (this.config.debug) {
            console.error('Detailed error information:', {
                message: error.message,
                stack: error.stack,
                timestamp: new Date().toISOString()
            });
        }
    }

    /**
     * Get application status
     */
    getStatus() {
        return {
            isInitialized: this.isInitialized,
            isRunning: this.isRunning,
            version: this.config.version,
            uptime: Date.now() - this.startTime,
            components: {
                apiClient: this.apiClient.isConnected(),
                dataStore: this.dataStore.isReady(),
                userManager: this.userManager.isInitialized()
            }
        };
    }

    /**
     * Update application configuration
     */
    updateConfig(newConfig) {
        const oldConfig = { ...this.config };
        this.config = { ...this.config, ...newConfig };

        this.logger.info('Configuration updated', {
            changed: Object.keys(newConfig),
            from: oldConfig,
            to: this.config
        });

        this.emit('config-updated', { old: oldConfig, new: this.config });
    }
}

export function createApplication(config) {
    return new Application(config);
}

let defaultApp = null;

export function getDefaultApp() {
    if (!defaultApp) {
        defaultApp = new Application();
    }
    return defaultApp;
}

if (typeof window !== 'undefined') {
    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', async () => {
            try {
                const app = getDefaultApp();
                await app.start();
                window.app = app;
            } catch (error) {
                console.error('Failed to auto-start application:', error);
            }
        });
    } else {
        setTimeout(async () => {
            try {
                const app = getDefaultApp();
                await app.start();
                window.app = app;
            } catch (error) {
                console.error('Failed to auto-start application:', error);
            }
        }, 0);
    }

    if (APP_CONFIG.debug) {
        window.AppUtils = Utils;
        window.createApplication = createApplication;
    }
}

if (typeof module !== 'undefined' && module.exports) {
    module.exports = {
        Application,
        createApplication,
        getDefaultApp,
        APP_CONFIG
    };
}

export { Application, APP_CONFIG };
export default Application;