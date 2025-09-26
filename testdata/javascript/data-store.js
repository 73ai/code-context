/**
 * Data storage and state management for the JavaScript test project.
 *
 * This module provides:
 * - Local storage abstraction
 * - Session storage management
 * - In-memory data store
 * - Data persistence and synchronization
 * - Event-driven state management
 * - Caching with TTL support
 */

import { EventEmitter } from 'events';

/**
 * Storage adapter interface
 */
export class StorageAdapter {
    async get(key) {
        throw new Error('get method must be implemented');
    }

    async set(key, value) {
        throw new Error('set method must be implemented');
    }

    async remove(key) {
        throw new Error('remove method must be implemented');
    }

    async clear() {
        throw new Error('clear method must be implemented');
    }

    async keys() {
        throw new Error('keys method must be implemented');
    }
}

/**
 * LocalStorage adapter
 */
export class LocalStorageAdapter extends StorageAdapter {
    constructor(prefix = 'app_') {
        super();
        this.prefix = prefix;
        this.isAvailable = this.checkAvailability();
    }

    checkAvailability() {
        try {
            const testKey = '__storage_test__';
            localStorage.setItem(testKey, 'test');
            localStorage.removeItem(testKey);
            return true;
        } catch (e) {
            return false;
        }
    }

    getKey(key) {
        return `${this.prefix}${key}`;
    }

    async get(key) {
        if (!this.isAvailable) return null;

        try {
            const item = localStorage.getItem(this.getKey(key));
            return item ? JSON.parse(item) : null;
        } catch (error) {
            console.warn(`Failed to get item from localStorage: ${key}`, error);
            return null;
        }
    }

    async set(key, value) {
        if (!this.isAvailable) return false;

        try {
            localStorage.setItem(this.getKey(key), JSON.stringify(value));
            return true;
        } catch (error) {
            console.warn(`Failed to set item in localStorage: ${key}`, error);
            return false;
        }
    }

    async remove(key) {
        if (!this.isAvailable) return false;

        try {
            localStorage.removeItem(this.getKey(key));
            return true;
        } catch (error) {
            console.warn(`Failed to remove item from localStorage: ${key}`, error);
            return false;
        }
    }

    async clear() {
        if (!this.isAvailable) return false;

        try {
            const keys = await this.keys();
            keys.forEach(key => localStorage.removeItem(key));
            return true;
        } catch (error) {
            console.warn('Failed to clear localStorage', error);
            return false;
        }
    }

    async keys() {
        if (!this.isAvailable) return [];

        const keys = [];
        for (let i = 0; i < localStorage.length; i++) {
            const key = localStorage.key(i);
            if (key && key.startsWith(this.prefix)) {
                keys.push(key);
            }
        }
        return keys;
    }
}

/**
 * SessionStorage adapter
 */
export class SessionStorageAdapter extends StorageAdapter {
    constructor(prefix = 'app_') {
        super();
        this.prefix = prefix;
        this.isAvailable = this.checkAvailability();
    }

    checkAvailability() {
        try {
            const testKey = '__storage_test__';
            sessionStorage.setItem(testKey, 'test');
            sessionStorage.removeItem(testKey);
            return true;
        } catch (e) {
            return false;
        }
    }

    getKey(key) {
        return `${this.prefix}${key}`;
    }

    async get(key) {
        if (!this.isAvailable) return null;

        try {
            const item = sessionStorage.getItem(this.getKey(key));
            return item ? JSON.parse(item) : null;
        } catch (error) {
            console.warn(`Failed to get item from sessionStorage: ${key}`, error);
            return null;
        }
    }

    async set(key, value) {
        if (!this.isAvailable) return false;

        try {
            sessionStorage.setItem(this.getKey(key), JSON.stringify(value));
            return true;
        } catch (error) {
            console.warn(`Failed to set item in sessionStorage: ${key}`, error);
            return false;
        }
    }

    async remove(key) {
        if (!this.isAvailable) return false;

        try {
            sessionStorage.removeItem(this.getKey(key));
            return true;
        } catch (error) {
            console.warn(`Failed to remove item from sessionStorage: ${key}`, error);
            return false;
        }
    }

    async clear() {
        if (!this.isAvailable) return false;

        try {
            const keys = await this.keys();
            keys.forEach(key => sessionStorage.removeItem(key));
            return true;
        } catch (error) {
            console.warn('Failed to clear sessionStorage', error);
            return false;
        }
    }

    async keys() {
        if (!this.isAvailable) return [];

        const keys = [];
        for (let i = 0; i < sessionStorage.length; i++) {
            const key = sessionStorage.key(i);
            if (key && key.startsWith(this.prefix)) {
                keys.push(key);
            }
        }
        return keys;
    }
}

/**
 * In-memory storage adapter
 */
export class MemoryStorageAdapter extends StorageAdapter {
    constructor() {
        super();
        this.data = new Map();
    }

    async get(key) {
        const item = this.data.get(key);
        return item ? JSON.parse(JSON.stringify(item)) : null;
    }

    async set(key, value) {
        this.data.set(key, JSON.parse(JSON.stringify(value)));
        return true;
    }

    async remove(key) {
        return this.data.delete(key);
    }

    async clear() {
        this.data.clear();
        return true;
    }

    async keys() {
        return Array.from(this.data.keys());
    }

    size() {
        return this.data.size;
    }
}

/**
 * Cache item with TTL support
 */
class CacheItem {
    constructor(value, ttl = null) {
        this.value = value;
        this.createdAt = Date.now();
        this.expiresAt = ttl ? this.createdAt + ttl : null;
        this.lastAccessed = this.createdAt;
        this.accessCount = 0;
    }

    isExpired() {
        return this.expiresAt && Date.now() > this.expiresAt;
    }

    touch() {
        this.lastAccessed = Date.now();
        this.accessCount++;
    }

    getRemainingTTL() {
        if (!this.expiresAt) return null;
        return Math.max(0, this.expiresAt - Date.now());
    }
}

/**
 * Main DataStore class with caching and persistence
 */
export class DataStore extends EventEmitter {
    constructor(options = {}) {
        super();

        this.options = {
            adapter: 'localStorage',
            namespace: 'app',
            defaultTTL: 1000 * 60 * 60,
            maxCacheSize: 100,
            persistToDisk: true,
            autoCleanup: true,
            cleanupInterval: 1000 * 60 * 5,
            ...options
        };

        this.cache = new Map();
        this.isReady = false;
        this.pendingOperations = [];

        this.setupAdapter();
        this.setupCleanup();
    }

    setupAdapter() {
        const { adapter, namespace } = this.options;

        switch (adapter) {
            case 'localStorage':
                this.adapter = new LocalStorageAdapter(namespace + '_');
                break;
            case 'sessionStorage':
                this.adapter = new SessionStorageAdapter(namespace + '_');
                break;
            case 'memory':
                this.adapter = new MemoryStorageAdapter();
                break;
            default:
                if (typeof adapter === 'object' && adapter instanceof StorageAdapter) {
                    this.adapter = adapter;
                } else {
                    throw new Error(`Invalid storage adapter: ${adapter}`);
                }
        }
    }

    setupCleanup() {
        if (this.options.autoCleanup) {
            this.cleanupInterval = setInterval(() => {
                this.cleanup();
            }, this.options.cleanupInterval);
        }
    }

    async initialize() {
        if (this.isReady) return;

        try {
            if (this.options.persistToDisk) {
                await this.loadPersistedCache();
            }

            for (const operation of this.pendingOperations) {
                await operation();
            }
            this.pendingOperations = [];

            this.isReady = true;
            this.emit('ready');
        } catch (error) {
            this.emit('error', error);
            throw error;
        }
    }

    async loadPersistedCache() {
        try {
            const persistedData = await this.adapter.get('__cache__');
            if (persistedData) {
                for (const [key, itemData] of Object.entries(persistedData)) {
                    const item = new CacheItem(itemData.value, null);
                    item.createdAt = itemData.createdAt;
                    item.expiresAt = itemData.expiresAt;
                    item.lastAccessed = itemData.lastAccessed;
                    item.accessCount = itemData.accessCount;

                    if (!item.isExpired()) {
                        this.cache.set(key, item);
                    }
                }
            }
        } catch (error) {
            console.warn('Failed to load persisted cache:', error);
        }
    }

    async persistCache() {
        if (!this.options.persistToDisk) return;

        try {
            const cacheData = {};
            for (const [key, item] of this.cache.entries()) {
                if (!item.isExpired()) {
                    cacheData[key] = {
                        value: item.value,
                        createdAt: item.createdAt,
                        expiresAt: item.expiresAt,
                        lastAccessed: item.lastAccessed,
                        accessCount: item.accessCount
                    };
                }
            }
            await this.adapter.set('__cache__', cacheData);
        } catch (error) {
            console.warn('Failed to persist cache:', error);
        }
    }

    async get(key, defaultValue = null) {
        if (!this.isReady) {
            return new Promise((resolve) => {
                this.pendingOperations.push(async () => {
                    resolve(await this.get(key, defaultValue));
                });
            });
        }

        try {
            if (this.cache.has(key)) {
                const item = this.cache.get(key);
                if (item.isExpired()) {
                    this.cache.delete(key);
                    await this.adapter.remove(key);
                } else {
                    item.touch();
                    this.emit('cache-hit', key);
                    return item.value;
                }
            }

            const value = await this.adapter.get(key);
            if (value !== null) {
                const item = new CacheItem(value);
                this.setCacheItem(key, item);
                this.emit('cache-miss', key);
                return value;
            }

            this.emit('cache-miss', key);
            return defaultValue;
        } catch (error) {
            this.emit('error', { operation: 'get', key, error });
            return defaultValue;
        }
    }

    async set(key, value, ttl = this.options.defaultTTL) {
        if (!this.isReady) {
            return new Promise((resolve) => {
                this.pendingOperations.push(async () => {
                    resolve(await this.set(key, value, ttl));
                });
            });
        }

        try {
            const item = new CacheItem(value, ttl);
            this.setCacheItem(key, item);

            const success = await this.adapter.set(key, value);

            if (success) {
                this.emit('data-changed', { key, value, operation: 'set' });
                await this.persistCache();
            }

            return success;
        } catch (error) {
            this.emit('error', { operation: 'set', key, error });
            return false;
        }
    }

    async remove(key) {
        if (!this.isReady) {
            return new Promise((resolve) => {
                this.pendingOperations.push(async () => {
                    resolve(await this.remove(key));
                });
            });
        }

        try {
            this.cache.delete(key);
            const success = await this.adapter.remove(key);

            if (success) {
                this.emit('data-changed', { key, operation: 'remove' });
                await this.persistCache();
            }

            return success;
        } catch (error) {
            this.emit('error', { operation: 'remove', key, error });
            return false;
        }
    }

    async clear() {
        try {
            this.cache.clear();
            const success = await this.adapter.clear();

            if (success) {
                this.emit('data-changed', { operation: 'clear' });
            }

            return success;
        } catch (error) {
            this.emit('error', { operation: 'clear', error });
            return false;
        }
    }

    async has(key) {
        const value = await this.get(key);
        return value !== null;
    }

    async keys() {
        try {
            const persistentKeys = await this.adapter.keys();
            const cacheKeys = Array.from(this.cache.keys());
            return [...new Set([...persistentKeys, ...cacheKeys])].filter(key => key !== '__cache__');
        } catch (error) {
            this.emit('error', { operation: 'keys', error });
            return [];
        }
    }

    async getAllData() {
        try {
            const keys = await this.keys();
            const data = {};

            for (const key of keys) {
                data[key] = await this.get(key);
            }

            return data;
        } catch (error) {
            this.emit('error', { operation: 'getAllData', error });
            return {};
        }
    }

    async batchUpdate(updates) {
        try {
            const results = [];

            for (const [key, value] of Object.entries(updates)) {
                const success = await this.set(key, value);
                results.push({ key, success });
            }

            this.emit('batch-updated', { updates, results });
            return results;
        } catch (error) {
            this.emit('error', { operation: 'batchUpdate', error });
            return [];
        }
    }

    setCacheItem(key, item) {
        if (this.cache.size >= this.options.maxCacheSize) {
            let lruKey = null;
            let lruTime = Date.now();

            for (const [k, v] of this.cache.entries()) {
                if (v.lastAccessed < lruTime) {
                    lruTime = v.lastAccessed;
                    lruKey = k;
                }
            }

            if (lruKey) {
                this.cache.delete(lruKey);
            }
        }

        this.cache.set(key, item);
    }

    cleanup() {
        let cleaned = 0;

        for (const [key, item] of this.cache.entries()) {
            if (item.isExpired()) {
                this.cache.delete(key);
                this.adapter.remove(key);
                cleaned++;
            }
        }

        if (cleaned > 0) {
            this.emit('cleanup', { itemsRemoved: cleaned });
            this.persistCache();
        }
    }

    cleanupExpired() {
        this.cleanup();
    }

    getStats() {
        const now = Date.now();
        const stats = {
            cacheSize: this.cache.size,
            maxCacheSize: this.options.maxCacheSize,
            totalItems: 0,
            expiredItems: 0,
            oldestItem: null,
            newestItem: null,
            totalAccessCount: 0,
            averageAge: 0
        };

        let totalAge = 0;
        let oldestTime = now;
        let newestTime = 0;

        for (const [key, item] of this.cache.entries()) {
            stats.totalItems++;
            stats.totalAccessCount += item.accessCount;

            const age = now - item.createdAt;
            totalAge += age;

            if (item.createdAt < oldestTime) {
                oldestTime = item.createdAt;
                stats.oldestItem = key;
            }

            if (item.createdAt > newestTime) {
                newestTime = item.createdAt;
                stats.newestItem = key;
            }

            if (item.isExpired()) {
                stats.expiredItems++;
            }
        }

        if (stats.totalItems > 0) {
            stats.averageAge = totalAge / stats.totalItems;
        }

        return stats;
    }

    async cleanup() {
        if (this.cleanupInterval) {
            clearInterval(this.cleanupInterval);
        }

        await this.persistCache();
        this.cache.clear();
        this.emit('cleanup-complete');
    }
}

/**
 * State manager for reactive data handling
 */
export class StateManager extends EventEmitter {
    constructor(initialState = {}) {
        super();

        this.state = { ...initialState };
        this.mutations = new Map();
        this.actions = new Map();
        this.watchers = new Map();
    }

    getState() {
        return { ...this.state };
    }

    getItem(key) {
        return this.state[key];
    }

    setState(newState) {
        const oldState = { ...this.state };
        this.state = { ...this.state, ...newState };

        for (const [key, value] of Object.entries(newState)) {
            if (oldState[key] !== value) {
                this.notifyWatchers(key, value, oldState[key]);
            }
        }

        this.emit('state-changed', { oldState, newState: this.state });
    }

    setItem(key, value) {
        this.setState({ [key]: value });
    }

    registerMutation(name, mutationFn) {
        this.mutations.set(name, mutationFn);
    }

    registerAction(name, actionFn) {
        this.actions.set(name, actionFn);
    }

    commit(mutationName, payload) {
        const mutation = this.mutations.get(mutationName);
        if (!mutation) {
            throw new Error(`Mutation '${mutationName}' not found`);
        }

        const oldState = { ...this.state };
        mutation(this.state, payload);
        this.emit('mutation', { name: mutationName, payload, oldState, newState: this.state });

        for (const key of Object.keys(this.state)) {
            if (oldState[key] !== this.state[key]) {
                this.notifyWatchers(key, this.state[key], oldState[key]);
            }
        }
    }

    async dispatch(actionName, payload) {
        const action = this.actions.get(actionName);
        if (!action) {
            throw new Error(`Action '${actionName}' not found`);
        }

        const context = {
            state: this.state,
            commit: this.commit.bind(this),
            dispatch: this.dispatch.bind(this)
        };

        this.emit('action-start', { name: actionName, payload });

        try {
            const result = await action(context, payload);
            this.emit('action-complete', { name: actionName, payload, result });
            return result;
        } catch (error) {
            this.emit('action-error', { name: actionName, payload, error });
            throw error;
        }
    }

    watch(key, callback) {
        if (!this.watchers.has(key)) {
            this.watchers.set(key, new Set());
        }
        this.watchers.get(key).add(callback);

        return () => {
            const watchers = this.watchers.get(key);
            if (watchers) {
                watchers.delete(callback);
                if (watchers.size === 0) {
                    this.watchers.delete(key);
                }
            }
        };
    }

    notifyWatchers(key, newValue, oldValue) {
        const watchers = this.watchers.get(key);
        if (watchers) {
            watchers.forEach(callback => {
                try {
                    callback(newValue, oldValue, key);
                } catch (error) {
                    console.error('Error in state watcher:', error);
                }
            });
        }
    }
}

export function createDataStore(options) {
    return new DataStore(options);
}

export function createStateManager(initialState) {
    return new StateManager(initialState);
}

export { DataStore, StateManager };
export default DataStore;