/**
 * Utility functions and helper classes for the JavaScript test project.
 *
 * This module provides:
 * - String manipulation utilities
 * - Data validation functions
 * - Date/time helpers
 * - Object manipulation utilities
 * - Array processing functions
 * - DOM utilities (browser-specific)
 * - Async utilities and helpers
 * - Mathematical calculations
 * - URL and query string handling
 */

/**
 * String manipulation utilities
 */
export class StringUtils {
    /**
     * Convert string to camelCase
     */
    static toCamelCase(str) {
        return str
            .toLowerCase()
            .replace(/[^a-zA-Z0-9]+(.)/g, (_, char) => char.toUpperCase());
    }

    /**
     * Convert string to kebab-case
     */
    static toKebabCase(str) {
        return str
            .replace(/([a-z])([A-Z])/g, '$1-$2')
            .replace(/[\s_]+/g, '-')
            .toLowerCase();
    }

    /**
     * Convert string to snake_case
     */
    static toSnakeCase(str) {
        return str
            .replace(/([a-z])([A-Z])/g, '$1_$2')
            .replace(/[\s-]+/g, '_')
            .toLowerCase();
    }

    /**
     * Capitalize first letter of each word
     */
    static toTitleCase(str) {
        return str
            .toLowerCase()
            .split(' ')
            .map(word => word.charAt(0).toUpperCase() + word.slice(1))
            .join(' ');
    }

    /**
     * Truncate string to specified length
     */
    static truncate(str, length, suffix = '...') {
        if (str.length <= length) return str;
        return str.substring(0, length - suffix.length) + suffix;
    }

    /**
     * Remove HTML tags from string
     */
    static stripHtml(str) {
        return str.replace(/<[^>]*>/g, '');
    }

    /**
     * Generate URL-friendly slug
     */
    static slugify(str, maxLength = 50) {
        return str
            .toLowerCase()
            .trim()
            .replace(/[^\w\s-]/g, '')
            .replace(/[\s_-]+/g, '-')
            .replace(/^-+|-+$/g, '')
            .substring(0, maxLength);
    }

    /**
     * Escape HTML characters
     */
    static escapeHtml(str) {
        const div = document?.createElement('div');
        if (div) {
            div.textContent = str;
            return div.innerHTML;
        }

        // Fallback for Node.js
        return str
            .replace(/&/g, '&amp;')
            .replace(/</g, '&lt;')
            .replace(/>/g, '&gt;')
            .replace(/"/g, '&quot;')
            .replace(/'/g, '&#39;');
    }

    /**
     * Generate random string
     */
    static generateRandomString(length = 10, charset = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789') {
        let result = '';
        for (let i = 0; i < length; i++) {
            result += charset.charAt(Math.floor(Math.random() * charset.length));
        }
        return result;
    }

    /**
     * Count occurrences of substring
     */
    static countOccurrences(str, substring) {
        return (str.match(new RegExp(substring, 'g')) || []).length;
    }

    /**
     * Replace multiple spaces with single space
     */
    static normalizeWhitespace(str) {
        return str.replace(/\s+/g, ' ').trim();
    }
}

/**
 * Validation utilities
 */
export class ValidationUtils {
    /**
     * Email validation regex
     */
    static EMAIL_REGEX = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;

    /**
     * Phone number patterns
     */
    static PHONE_PATTERNS = {
        US: /^\+?1?[-.\s]?\(?([0-9]{3})\)?[-.\s]?([0-9]{3})[-.\s]?([0-9]{4})$/,
        INTERNATIONAL: /^\+?[1-9]\d{1,14}$/
    };

    /**
     * URL validation regex
     */
    static URL_REGEX = /^https?:\/\/(www\.)?[-a-zA-Z0-9@:%._\+~#=]{1,256}\.[a-zA-Z0-9()]{1,6}\b([-a-zA-Z0-9()@:%_\+.~#?&//=]*)$/;

    /**
     * Validate email address
     */
    static isValidEmail(email) {
        if (typeof email !== 'string') return false;
        return this.EMAIL_REGEX.test(email.trim());
    }

    /**
     * Validate phone number
     */
    static isValidPhone(phone, region = 'US') {
        if (typeof phone !== 'string') return false;
        const pattern = this.PHONE_PATTERNS[region] || this.PHONE_PATTERNS.INTERNATIONAL;
        return pattern.test(phone.replace(/\s+/g, ''));
    }

    /**
     * Validate URL
     */
    static isValidUrl(url) {
        if (typeof url !== 'string') return false;
        return this.URL_REGEX.test(url);
    }

    /**
     * Validate credit card using Luhn algorithm
     */
    static isValidCreditCard(cardNumber) {
        if (typeof cardNumber !== 'string') return false;

        const cleanNumber = cardNumber.replace(/\D/g, '');
        if (cleanNumber.length < 13 || cleanNumber.length > 19) return false;

        let sum = 0;
        let isEven = false;

        for (let i = cleanNumber.length - 1; i >= 0; i--) {
            let digit = parseInt(cleanNumber[i]);

            if (isEven) {
                digit *= 2;
                if (digit > 9) digit -= 9;
            }

            sum += digit;
            isEven = !isEven;
        }

        return sum % 10 === 0;
    }

    /**
     * Validate password strength
     */
    static validatePasswordStrength(password) {
        if (typeof password !== 'string') {
            return { isValid: false, score: 0, feedback: ['Password must be a string'] };
        }

        const feedback = [];
        let score = 0;

        // Length check
        if (password.length < 8) {
            feedback.push('Password must be at least 8 characters long');
        } else {
            score += 1;
            if (password.length >= 12) score += 1;
        }

        // Character variety checks
        if (!/[a-z]/.test(password)) {
            feedback.push('Password must contain lowercase letters');
        } else {
            score += 1;
        }

        if (!/[A-Z]/.test(password)) {
            feedback.push('Password must contain uppercase letters');
        } else {
            score += 1;
        }

        if (!/\d/.test(password)) {
            feedback.push('Password must contain numbers');
        } else {
            score += 1;
        }

        if (!/[!@#$%^&*(),.?":{}|<>]/.test(password)) {
            feedback.push('Password must contain special characters');
        } else {
            score += 1;
        }

        // Common patterns
        if (/(.)\1{2,}/.test(password)) {
            feedback.push('Avoid repeating characters');
            score -= 1;
        }

        if (/123|abc|qwe/i.test(password)) {
            feedback.push('Avoid sequential characters');
            score -= 1;
        }

        return {
            isValid: feedback.length === 0,
            score: Math.max(0, Math.min(5, score)),
            feedback
        };
    }

    /**
     * Sanitize input string
     */
    static sanitizeInput(input, maxLength = 1000) {
        if (typeof input !== 'string') return '';

        return input
            .trim()
            .replace(/[<>]/g, '') // Remove potential HTML tags
            .substring(0, maxLength);
    }
}

/**
 * Date and time utilities
 */
export class DateUtils {
    /**
     * Format date in various formats
     */
    static formatDate(date, format = 'YYYY-MM-DD') {
        if (!(date instanceof Date)) return '';

        const year = date.getFullYear();
        const month = String(date.getMonth() + 1).padStart(2, '0');
        const day = String(date.getDate()).padStart(2, '0');
        const hours = String(date.getHours()).padStart(2, '0');
        const minutes = String(date.getMinutes()).padStart(2, '0');
        const seconds = String(date.getSeconds()).padStart(2, '0');

        const formats = {
            'YYYY-MM-DD': `${year}-${month}-${day}`,
            'DD/MM/YYYY': `${day}/${month}/${year}`,
            'MM/DD/YYYY': `${month}/${day}/${year}`,
            'YYYY-MM-DD HH:mm:ss': `${year}-${month}-${day} ${hours}:${minutes}:${seconds}`,
            'DD MMM YYYY': `${day} ${this.getMonthName(date.getMonth())} ${year}`,
            'MMM DD, YYYY': `${this.getMonthName(date.getMonth())} ${day}, ${year}`
        };

        return formats[format] || date.toString();
    }

    /**
     * Get month name
     */
    static getMonthName(monthIndex, short = true) {
        const months = short
            ? ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec']
            : ['January', 'February', 'March', 'April', 'May', 'June',
               'July', 'August', 'September', 'October', 'November', 'December'];
        return months[monthIndex] || '';
    }

    /**
     * Calculate age from birth date
     */
    static calculateAge(birthDate) {
        if (!(birthDate instanceof Date)) return 0;

        const today = new Date();
        let age = today.getFullYear() - birthDate.getFullYear();
        const monthDiff = today.getMonth() - birthDate.getMonth();

        if (monthDiff < 0 || (monthDiff === 0 && today.getDate() < birthDate.getDate())) {
            age--;
        }

        return Math.max(0, age);
    }

    /**
     * Get relative time string (e.g., "2 hours ago")
     */
    static getRelativeTime(date) {
        if (!(date instanceof Date)) return '';

        const now = new Date();
        const diffMs = now - date;
        const diffSeconds = Math.floor(diffMs / 1000);
        const diffMinutes = Math.floor(diffSeconds / 60);
        const diffHours = Math.floor(diffMinutes / 60);
        const diffDays = Math.floor(diffHours / 24);

        if (diffSeconds < 60) return 'just now';
        if (diffMinutes < 60) return `${diffMinutes} minute${diffMinutes !== 1 ? 's' : ''} ago`;
        if (diffHours < 24) return `${diffHours} hour${diffHours !== 1 ? 's' : ''} ago`;
        if (diffDays < 7) return `${diffDays} day${diffDays !== 1 ? 's' : ''} ago`;
        if (diffDays < 30) {
            const weeks = Math.floor(diffDays / 7);
            return `${weeks} week${weeks !== 1 ? 's' : ''} ago`;
        }
        if (diffDays < 365) {
            const months = Math.floor(diffDays / 30);
            return `${months} month${months !== 1 ? 's' : ''} ago`;
        }

        const years = Math.floor(diffDays / 365);
        return `${years} year${years !== 1 ? 's' : ''} ago`;
    }

    /**
     * Check if date is weekend
     */
    static isWeekend(date) {
        if (!(date instanceof Date)) return false;
        const day = date.getDay();
        return day === 0 || day === 6; // Sunday or Saturday
    }

    /**
     * Add days to date
     */
    static addDays(date, days) {
        if (!(date instanceof Date)) return null;
        const result = new Date(date);
        result.setDate(result.getDate() + days);
        return result;
    }

    /**
     * Get start of day
     */
    static startOfDay(date) {
        if (!(date instanceof Date)) return null;
        const result = new Date(date);
        result.setHours(0, 0, 0, 0);
        return result;
    }

    /**
     * Get end of day
     */
    static endOfDay(date) {
        if (!(date instanceof Date)) return null;
        const result = new Date(date);
        result.setHours(23, 59, 59, 999);
        return result;
    }
}

/**
 * Object manipulation utilities
 */
export class ObjectUtils {
    /**
     * Deep clone object
     */
    static deepClone(obj) {
        if (obj === null || typeof obj !== 'object') return obj;
        if (obj instanceof Date) return new Date(obj);
        if (obj instanceof Array) return obj.map(item => this.deepClone(item));

        const cloned = {};
        for (const key in obj) {
            if (obj.hasOwnProperty(key)) {
                cloned[key] = this.deepClone(obj[key]);
            }
        }
        return cloned;
    }

    /**
     * Deep merge objects
     */
    static deepMerge(target, ...sources) {
        if (!sources.length) return target;
        const source = sources.shift();

        if (this.isObject(target) && this.isObject(source)) {
            for (const key in source) {
                if (this.isObject(source[key])) {
                    if (!target[key]) Object.assign(target, { [key]: {} });
                    this.deepMerge(target[key], source[key]);
                } else {
                    Object.assign(target, { [key]: source[key] });
                }
            }
        }

        return this.deepMerge(target, ...sources);
    }

    /**
     * Check if value is an object
     */
    static isObject(item) {
        return item && typeof item === 'object' && !Array.isArray(item);
    }

    /**
     * Get nested property value
     */
    static getNestedProperty(obj, path, defaultValue = undefined) {
        const keys = path.split('.');
        let current = obj;

        for (const key of keys) {
            if (current === null || current === undefined || !(key in current)) {
                return defaultValue;
            }
            current = current[key];
        }

        return current;
    }

    /**
     * Set nested property value
     */
    static setNestedProperty(obj, path, value) {
        const keys = path.split('.');
        let current = obj;

        for (let i = 0; i < keys.length - 1; i++) {
            const key = keys[i];
            if (!(key in current) || !this.isObject(current[key])) {
                current[key] = {};
            }
            current = current[key];
        }

        current[keys[keys.length - 1]] = value;
        return obj;
    }

    /**
     * Flatten nested object
     */
    static flatten(obj, prefix = '', separator = '.') {
        const result = {};

        for (const key in obj) {
            if (obj.hasOwnProperty(key)) {
                const newKey = prefix ? `${prefix}${separator}${key}` : key;

                if (this.isObject(obj[key])) {
                    Object.assign(result, this.flatten(obj[key], newKey, separator));
                } else {
                    result[newKey] = obj[key];
                }
            }
        }

        return result;
    }

    /**
     * Pick specific properties from object
     */
    static pick(obj, keys) {
        const result = {};
        for (const key of keys) {
            if (key in obj) {
                result[key] = obj[key];
            }
        }
        return result;
    }

    /**
     * Omit specific properties from object
     */
    static omit(obj, keys) {
        const result = { ...obj };
        for (const key of keys) {
            delete result[key];
        }
        return result;
    }
}

/**
 * Array processing utilities
 */
export class ArrayUtils {
    /**
     * Remove duplicates from array
     */
    static unique(array) {
        return [...new Set(array)];
    }

    /**
     * Chunk array into smaller arrays
     */
    static chunk(array, size) {
        const chunks = [];
        for (let i = 0; i < array.length; i += size) {
            chunks.push(array.slice(i, i + size));
        }
        return chunks;
    }

    /**
     * Shuffle array using Fisher-Yates algorithm
     */
    static shuffle(array) {
        const shuffled = [...array];
        for (let i = shuffled.length - 1; i > 0; i--) {
            const j = Math.floor(Math.random() * (i + 1));
            [shuffled[i], shuffled[j]] = [shuffled[j], shuffled[i]];
        }
        return shuffled;
    }

    /**
     * Group array items by key
     */
    static groupBy(array, keyOrFn) {
        const keyFn = typeof keyOrFn === 'function' ? keyOrFn : item => item[keyOrFn];
        return array.reduce((groups, item) => {
            const key = keyFn(item);
            if (!groups[key]) groups[key] = [];
            groups[key].push(item);
            return groups;
        }, {});
    }

    /**
     * Sort array by multiple criteria
     */
    static sortBy(array, ...criteria) {
        return [...array].sort((a, b) => {
            for (const criterion of criteria) {
                let aVal, bVal;

                if (typeof criterion === 'string') {
                    aVal = a[criterion];
                    bVal = b[criterion];
                } else if (typeof criterion === 'function') {
                    aVal = criterion(a);
                    bVal = criterion(b);
                } else {
                    continue;
                }

                if (aVal < bVal) return -1;
                if (aVal > bVal) return 1;
            }
            return 0;
        });
    }

    /**
     * Find intersection of arrays
     */
    static intersection(...arrays) {
        if (arrays.length === 0) return [];
        return arrays.reduce((acc, array) => acc.filter(item => array.includes(item)));
    }

    /**
     * Find difference between arrays
     */
    static difference(array1, array2) {
        return array1.filter(item => !array2.includes(item));
    }
}

/**
 * Mathematical utilities
 */
export class MathUtils {
    /**
     * Clamp value between min and max
     */
    static clamp(value, min, max) {
        return Math.min(Math.max(value, min), max);
    }

    /**
     * Linear interpolation
     */
    static lerp(start, end, t) {
        return start + (end - start) * t;
    }

    /**
     * Map value from one range to another
     */
    static mapRange(value, inMin, inMax, outMin, outMax) {
        return ((value - inMin) * (outMax - outMin)) / (inMax - inMin) + outMin;
    }

    /**
     * Calculate average
     */
    static average(numbers) {
        if (!Array.isArray(numbers) || numbers.length === 0) return 0;
        return numbers.reduce((sum, num) => sum + num, 0) / numbers.length;
    }

    /**
     * Calculate median
     */
    static median(numbers) {
        if (!Array.isArray(numbers) || numbers.length === 0) return 0;

        const sorted = [...numbers].sort((a, b) => a - b);
        const mid = Math.floor(sorted.length / 2);

        return sorted.length % 2 === 0
            ? (sorted[mid - 1] + sorted[mid]) / 2
            : sorted[mid];
    }

    /**
     * Generate random number between min and max
     */
    static random(min = 0, max = 1) {
        return Math.random() * (max - min) + min;
    }

    /**
     * Generate random integer between min and max (inclusive)
     */
    static randomInt(min, max) {
        return Math.floor(Math.random() * (max - min + 1)) + min;
    }

    /**
     * Round to specified decimal places
     */
    static roundTo(number, decimals) {
        const factor = Math.pow(10, decimals);
        return Math.round(number * factor) / factor;
    }
}

/**
 * Async utilities
 */
export class AsyncUtils {
    /**
     * Sleep for specified milliseconds
     */
    static sleep(ms) {
        return new Promise(resolve => setTimeout(resolve, ms));
    }

    /**
     * Retry async function with exponential backoff
     */
    static async retry(fn, maxAttempts = 3, baseDelay = 1000) {
        let lastError;

        for (let attempt = 1; attempt <= maxAttempts; attempt++) {
            try {
                return await fn();
            } catch (error) {
                lastError = error;
                if (attempt === maxAttempts) break;

                const delay = baseDelay * Math.pow(2, attempt - 1);
                await this.sleep(delay);
            }
        }

        throw lastError;
    }

    /**
     * Create debounced function
     */
    static debounce(func, delay) {
        let timeoutId;
        return function (...args) {
            clearTimeout(timeoutId);
            timeoutId = setTimeout(() => func.apply(this, args), delay);
        };
    }

    /**
     * Create throttled function
     */
    static throttle(func, limit) {
        let inThrottle;
        return function (...args) {
            if (!inThrottle) {
                func.apply(this, args);
                inThrottle = true;
                setTimeout(() => (inThrottle = false), limit);
            }
        };
    }

    /**
     * Run promises with limited concurrency
     */
    static async concurrent(promises, limit = 5) {
        const results = [];
        const executing = [];

        for (const [index, promise] of promises.entries()) {
            const p = Promise.resolve(promise).then(result => ({ index, result }));
            results.push(p);

            if (promises.length >= limit) {
                executing.push(p);

                if (executing.length >= limit) {
                    await Promise.race(executing);
                    executing.splice(executing.findIndex(p => p.settled), 1);
                }
            }
        }

        const settled = await Promise.allSettled(results);
        return settled
            .sort((a, b) => a.value.index - b.value.index)
            .map(result => result.status === 'fulfilled' ? result.value.result : result.reason);
    }
}

/**
 * URL and query string utilities
 */
export class UrlUtils {
    /**
     * Parse query string into object
     */
    static parseQueryString(queryString) {
        const params = new URLSearchParams(queryString.startsWith('?') ? queryString.slice(1) : queryString);
        const result = {};

        for (const [key, value] of params) {
            if (key in result) {
                if (Array.isArray(result[key])) {
                    result[key].push(value);
                } else {
                    result[key] = [result[key], value];
                }
            } else {
                result[key] = value;
            }
        }

        return result;
    }

    /**
     * Build query string from object
     */
    static buildQueryString(params) {
        const searchParams = new URLSearchParams();

        for (const [key, value] of Object.entries(params)) {
            if (Array.isArray(value)) {
                value.forEach(v => searchParams.append(key, v));
            } else if (value !== null && value !== undefined) {
                searchParams.append(key, value);
            }
        }

        return searchParams.toString();
    }

    /**
     * Join URL parts
     */
    static joinUrl(...parts) {
        return parts
            .map((part, index) => {
                if (index === 0) return part.replace(/\/+$/, '');
                return part.replace(/^\/+|\/+$/g, '');
            })
            .filter(part => part.length > 0)
            .join('/');
    }
}

// DOM utilities (browser only)
export class DomUtils {
    /**
     * Check if element is in viewport
     */
    static isInViewport(element) {
        if (typeof window === 'undefined') return false;

        const rect = element.getBoundingClientRect();
        return (
            rect.top >= 0 &&
            rect.left >= 0 &&
            rect.bottom <= (window.innerHeight || document.documentElement.clientHeight) &&
            rect.right <= (window.innerWidth || document.documentElement.clientWidth)
        );
    }

    /**
     * Smooth scroll to element
     */
    static scrollToElement(element, offset = 0) {
        if (typeof window === 'undefined') return;

        const elementTop = element.getBoundingClientRect().top + window.pageYOffset;
        window.scrollTo({
            top: elementTop - offset,
            behavior: 'smooth'
        });
    }

    /**
     * Get element's computed style property
     */
    static getStyle(element, property) {
        if (typeof window === 'undefined') return null;
        return window.getComputedStyle(element).getPropertyValue(property);
    }
}

// Aggregate utils class
export class Utils {
    static String = StringUtils;
    static Validation = ValidationUtils;
    static Date = DateUtils;
    static Object = ObjectUtils;
    static Array = ArrayUtils;
    static Math = MathUtils;
    static Async = AsyncUtils;
    static Url = UrlUtils;
    static Dom = DomUtils;
}

// Default export
export default Utils;