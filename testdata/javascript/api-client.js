/**
 * API Client for handling HTTP requests and responses.
 *
 * Features:
 * - RESTful API methods (GET, POST, PUT, DELETE)
 * - Request/response interceptors
 * - Automatic retry logic
 * - Request/response caching
 * - Authentication handling
 * - Rate limiting
 */

import { EventEmitter } from 'events';

/**
 * HTTP methods enum
 */
export const HttpMethods = {
    GET: 'GET',
    POST: 'POST',
    PUT: 'PUT',
    DELETE: 'DELETE',
    PATCH: 'PATCH',
    HEAD: 'HEAD',
    OPTIONS: 'OPTIONS'
};

/**
 * Request configuration interface
 */
class RequestConfig {
    constructor({
        method = HttpMethods.GET,
        url = '',
        data = null,
        headers = {},
        timeout = 30000,
        retries = 3,
        cache = false,
        cacheTTL = 300000, // 5 minutes
        validateStatus = (status) => status >= 200 && status < 300
    } = {}) {
        this.method = method;
        this.url = url;
        this.data = data;
        this.headers = headers;
        this.timeout = timeout;
        this.retries = retries;
        this.cache = cache;
        this.cacheTTL = cacheTTL;
        this.validateStatus = validateStatus;
    }
}

/**
 * API Response wrapper
 */
class ApiResponse {
    constructor({ data, status, statusText, headers, config, request }) {
        this.data = data;
        this.status = status;
        this.statusText = statusText;
        this.headers = headers;
        this.config = config;
        this.request = request;
        this.timestamp = new Date();
    }

    get ok() {
        return this.status >= 200 && this.status < 300;
    }

    get isJson() {
        const contentType = this.headers.get?.('content-type') || this.headers['content-type'] || '';
        return contentType.includes('application/json');
    }
}

/**
 * API Error class
 */
export class ApiError extends Error {
    constructor(message, response = null, request = null) {
        super(message);
        this.name = 'ApiError';
        this.response = response;
        this.request = request;
        this.timestamp = new Date();
    }

    get status() {
        return this.response?.status || 0;
    }

    get isNetworkError() {
        return this.status === 0;
    }

    get isClientError() {
        return this.status >= 400 && this.status < 500;
    }

    get isServerError() {
        return this.status >= 500;
    }
}

/**
 * Request/Response Cache
 */
class RequestCache {
    constructor() {
        this.cache = new Map();
        this.maxSize = 100;
        this.defaultTTL = 300000; // 5 minutes
    }

    generateKey(config) {
        const { method, url, data } = config;
        const dataStr = data ? JSON.stringify(data) : '';
        return `${method}:${url}:${dataStr}`;
    }

    get(key) {
        const entry = this.cache.get(key);
        if (!entry) return null;

        if (Date.now() > entry.expiresAt) {
            this.cache.delete(key);
            return null;
        }

        return entry.response;
    }

    set(key, response, ttl = this.defaultTTL) {
        // Enforce max cache size
        if (this.cache.size >= this.maxSize) {
            const firstKey = this.cache.keys().next().value;
            this.cache.delete(firstKey);
        }

        this.cache.set(key, {
            response: response,
            expiresAt: Date.now() + ttl,
            createdAt: Date.now()
        });
    }

    clear() {
        this.cache.clear();
    }

    cleanup() {
        const now = Date.now();
        for (const [key, entry] of this.cache.entries()) {
            if (now > entry.expiresAt) {
                this.cache.delete(key);
            }
        }
    }

    getStats() {
        return {
            size: this.cache.size,
            maxSize: this.maxSize,
            entries: Array.from(this.cache.entries()).map(([key, entry]) => ({
                key,
                expiresAt: entry.expiresAt,
                isExpired: Date.now() > entry.expiresAt
            }))
        };
    }
}

/**
 * Rate Limiter
 */
class RateLimiter {
    constructor(maxRequests = 100, windowMs = 60000) {
        this.maxRequests = maxRequests;
        this.windowMs = windowMs;
        this.requests = [];
    }

    canMakeRequest() {
        const now = Date.now();
        const windowStart = now - this.windowMs;

        this.requests = this.requests.filter(time => time > windowStart);

        return this.requests.length < this.maxRequests;
    }

    recordRequest() {
        this.requests.push(Date.now());
    }

    getTimeUntilReset() {
        if (this.requests.length === 0) return 0;

        const oldestRequest = Math.min(...this.requests);
        const resetTime = oldestRequest + this.windowMs;
        return Math.max(0, resetTime - Date.now());
    }
}

/**
 * Main API Client class
 */
export class ApiClient extends EventEmitter {
    constructor(baseURL = '', apiKey = '') {
        super();

        this.baseURL = baseURL;
        this.apiKey = apiKey;
        this.cache = new RequestCache();
        this.rateLimiter = new RateLimiter();

        this.defaults = {
            timeout: 30000,
            retries: 3,
            headers: {
                'Content-Type': 'application/json',
                'Accept': 'application/json'
            }
        };

        if (this.apiKey) {
            this.defaults.headers['Authorization'] = `Bearer ${this.apiKey}`;
        }

        this.requestInterceptors = [];
        this.responseInterceptors = [];

        this.activeRequests = new Map();
        this.requestId = 0;

        this.stats = {
            requests: 0,
            responses: 0,
            errors: 0,
            cacheHits: 0,
            cacheMisses: 0,
            retries: 0
        };

        this.isInitialized = false;
    }

    /**
     * Initialize the API client
     */
    async initialize() {
        if (this.isInitialized) return;

        try {
            await this.healthCheck();

            this.cacheCleanupInterval = setInterval(() => {
                this.cache.cleanup();
            }, 60000);

            this.isInitialized = true;
            this.emit('initialized');
        } catch (error) {
            throw new ApiError('Failed to initialize API client', null, null);
        }
    }

    /**
     * Cleanup resources
     */
    async cleanup() {
        if (this.cacheCleanupInterval) {
            clearInterval(this.cacheCleanupInterval);
        }

        for (const [id, controller] of this.activeRequests) {
            controller.abort();
        }
        this.activeRequests.clear();

        this.cache.clear();
        this.isInitialized = false;
        this.emit('cleanup');
    }

    /**
     * Add request interceptor
     */
    addRequestInterceptor(interceptor) {
        if (typeof interceptor === 'function') {
            this.requestInterceptors.push(interceptor);
        }
    }

    /**
     * Add response interceptor
     */
    addResponseInterceptor(interceptor) {
        if (typeof interceptor === 'function') {
            this.responseInterceptors.push(interceptor);
        }
    }

    /**
     * Create request configuration
     */
    createConfig(config) {
        const mergedConfig = new RequestConfig({
            ...this.defaults,
            ...config,
            headers: { ...this.defaults.headers, ...config.headers }
        });

        if (mergedConfig.url && !mergedConfig.url.startsWith('http')) {
            mergedConfig.url = this.baseURL.replace(/\/$/, '') + '/' + mergedConfig.url.replace(/^\//, '');
        }

        return mergedConfig;
    }

    /**
     * Apply request interceptors
     */
    async applyRequestInterceptors(config) {
        let processedConfig = config;

        for (const interceptor of this.requestInterceptors) {
            try {
                processedConfig = await interceptor(processedConfig);
            } catch (error) {
                throw new ApiError('Request interceptor failed', null, processedConfig);
            }
        }

        return processedConfig;
    }

    /**
     * Apply response interceptors
     */
    async applyResponseInterceptors(response) {
        let processedResponse = response;

        for (const interceptor of this.responseInterceptors) {
            try {
                processedResponse = await interceptor(processedResponse);
            } catch (error) {
                throw new ApiError('Response interceptor failed', response, null);
            }
        }

        return processedResponse;
    }

    /**
     * Main request method
     */
    async request(config) {
        const requestConfig = this.createConfig(config);
        const requestId = ++this.requestId;

        try {
            const processedConfig = await this.applyRequestInterceptors(requestConfig);

            if (!this.rateLimiter.canMakeRequest()) {
                const waitTime = this.rateLimiter.getTimeUntilReset();
                await this.sleep(waitTime);
            }

            if (processedConfig.cache && processedConfig.method === HttpMethods.GET) {
                const cacheKey = this.cache.generateKey(processedConfig);
                const cachedResponse = this.cache.get(cacheKey);

                if (cachedResponse) {
                    this.stats.cacheHits++;
                    this.emit('cache-hit', { config: processedConfig, response: cachedResponse });
                    return cachedResponse;
                }
                this.stats.cacheMisses++;
            }

            const response = await this.makeRequest(processedConfig, requestId);

            if (response.ok && processedConfig.cache && processedConfig.method === HttpMethods.GET) {
                const cacheKey = this.cache.generateKey(processedConfig);
                this.cache.set(cacheKey, response, processedConfig.cacheTTL);
            }

            const processedResponse = await this.applyResponseInterceptors(response);

            this.stats.responses++;
            this.emit('response', { config: processedConfig, response: processedResponse });

            return processedResponse;

        } catch (error) {
            this.stats.errors++;
            this.emit('error', { config: requestConfig, error });
            throw error;
        }
    }

    /**
     * Make the actual HTTP request with retry logic
     */
    async makeRequest(config, requestId, attempt = 1) {
        const controller = new AbortController();
        this.activeRequests.set(requestId, controller);

        try {
            this.stats.requests++;
            this.rateLimiter.recordRequest();

            this.emit('request-start', { config, requestId, attempt });

            const fetchOptions = {
                method: config.method,
                headers: config.headers,
                signal: controller.signal
            };

            if (config.data && config.method !== HttpMethods.GET) {
                fetchOptions.body = typeof config.data === 'string'
                    ? config.data
                    : JSON.stringify(config.data);
            }

            const timeoutPromise = new Promise((_, reject) => {
                setTimeout(() => {
                    controller.abort();
                    reject(new ApiError('Request timeout', null, config));
                }, config.timeout);
            });

            const fetchPromise = fetch(config.url, fetchOptions);
            const response = await Promise.race([fetchPromise, timeoutPromise]);

            let data;
            const contentType = response.headers.get('content-type') || '';

            if (contentType.includes('application/json')) {
                data = await response.json();
            } else if (contentType.includes('text/')) {
                data = await response.text();
            } else {
                data = await response.blob();
            }

            const apiResponse = new ApiResponse({
                data,
                status: response.status,
                statusText: response.statusText,
                headers: response.headers,
                config,
                request: { requestId, attempt }
            });

            if (!config.validateStatus(response.status)) {
                throw new ApiError(
                    `Request failed with status ${response.status}: ${response.statusText}`,
                    apiResponse,
                    config
                );
            }

            this.emit('request-complete', { config, response: apiResponse, requestId, attempt });
            return apiResponse;

        } catch (error) {
            this.emit('request-error', { config, error, requestId, attempt });

            if (attempt <= config.retries && this.shouldRetry(error)) {
                this.stats.retries++;
                await this.sleep(this.getRetryDelay(attempt));
                return this.makeRequest(config, requestId, attempt + 1);
            }

            throw error;

        } finally {
            this.activeRequests.delete(requestId);
        }
    }

    /**
     * Determine if request should be retried
     */
    shouldRetry(error) {
        if (error.name === 'AbortError') return false;
        if (error.response && error.response.status < 500) return false;
        return true;
    }

    /**
     * Calculate retry delay with exponential backoff
     */
    getRetryDelay(attempt) {
        return Math.min(1000 * Math.pow(2, attempt - 1), 10000);
    }

    /**
     * Sleep utility
     */
    sleep(ms) {
        return new Promise(resolve => setTimeout(resolve, ms));
    }

    /**
     * HTTP method helpers
     */
    async get(url, config = {}) {
        return this.request({ ...config, method: HttpMethods.GET, url });
    }

    async post(url, data = null, config = {}) {
        return this.request({ ...config, method: HttpMethods.POST, url, data });
    }

    async put(url, data = null, config = {}) {
        return this.request({ ...config, method: HttpMethods.PUT, url, data });
    }

    async delete(url, config = {}) {
        return this.request({ ...config, method: HttpMethods.DELETE, url });
    }

    async patch(url, data = null, config = {}) {
        return this.request({ ...config, method: HttpMethods.PATCH, url, data });
    }

    async head(url, config = {}) {
        return this.request({ ...config, method: HttpMethods.HEAD, url });
    }

    async options(url, config = {}) {
        return this.request({ ...config, method: HttpMethods.OPTIONS, url });
    }

    /**
     * Health check endpoint
     */
    async healthCheck() {
        try {
            await this.get('/health', { timeout: 5000, retries: 1 });
            return true;
        } catch (error) {
            return false;
        }
    }

    /**
     * Set authentication token
     */
    setAuthToken(token) {
        this.apiKey = token;
        this.defaults.headers['Authorization'] = `Bearer ${token}`;
        this.emit('auth-token-updated', token);
    }

    /**
     * Remove authentication token
     */
    clearAuthToken() {
        this.apiKey = '';
        delete this.defaults.headers['Authorization'];
        this.emit('auth-token-cleared');
    }

    /**
     * Check if client is connected/initialized
     */
    isConnected() {
        return this.isInitialized;
    }

    /**
     * Get client statistics
     */
    getStats() {
        return {
            ...this.stats,
            cache: this.cache.getStats(),
            activeRequests: this.activeRequests.size,
            rateLimit: {
                maxRequests: this.rateLimiter.maxRequests,
                windowMs: this.rateLimiter.windowMs,
                remaining: this.rateLimiter.maxRequests - this.rateLimiter.requests.length,
                resetTime: this.rateLimiter.getTimeUntilReset()
            }
        };
    }

    /**
     * Reset statistics
     */
    resetStats() {
        this.stats = {
            requests: 0,
            responses: 0,
            errors: 0,
            cacheHits: 0,
            cacheMisses: 0,
            retries: 0
        };
    }

    /**
     * Cancel all active requests
     */
    cancelAllRequests() {
        for (const [id, controller] of this.activeRequests) {
            controller.abort();
        }
        this.activeRequests.clear();
    }

    /**
     * Update base configuration
     */
    updateDefaults(newDefaults) {
        this.defaults = { ...this.defaults, ...newDefaults };
        this.emit('defaults-updated', this.defaults);
    }
}

export function createApiClient(baseURL, apiKey) {
    return new ApiClient(baseURL, apiKey);
}

export default ApiClient;