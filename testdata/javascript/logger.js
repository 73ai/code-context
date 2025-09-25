/**
 * Logging utility for the JavaScript test project.
 *
 * This module provides:
 * - Multi-level logging (debug, info, warn, error)
 * - Multiple output targets (console, file, remote)
 * - Log formatting and filtering
 * - Performance monitoring
 * - Error tracking and reporting
 */

/**
 * Log levels enumeration
 */
export const LogLevel = {
    DEBUG: 0,
    INFO: 1,
    WARN: 2,
    ERROR: 3,
    SILENT: 4
};

export const LogLevelNames = {
    [LogLevel.DEBUG]: 'DEBUG',
    [LogLevel.INFO]: 'INFO',
    [LogLevel.WARN]: 'WARN',
    [LogLevel.ERROR]: 'ERROR',
    [LogLevel.SILENT]: 'SILENT'
};

/**
 * Log entry data structure
 */
export class LogEntry {
    constructor(level, message, data = null, context = {}) {
        this.id = this.generateId();
        this.timestamp = new Date();
        this.level = level;
        this.levelName = LogLevelNames[level];
        this.message = message;
        this.data = data;
        this.context = { ...context };

        // Add stack trace for errors
        if (level === LogLevel.ERROR && data instanceof Error) {
            this.stack = data.stack;
        }

        // Browser-specific context
        if (typeof window !== 'undefined') {
            this.context.userAgent = navigator.userAgent;
            this.context.url = window.location.href;
        }

        // Node.js-specific context
        if (typeof process !== 'undefined') {
            this.context.pid = process.pid;
            this.context.platform = process.platform;
        }
    }

    generateId() {
        return `log_${Date.now()}_${Math.random().toString(36).substr(2, 9)}`;
    }

    toJSON() {
        return {
            id: this.id,
            timestamp: this.timestamp.toISOString(),
            level: this.level,
            levelName: this.levelName,
            message: this.message,
            data: this.serializeData(this.data),
            context: this.context,
            stack: this.stack
        };
    }

    serializeData(data) {
        try {
            if (data === null || data === undefined) return null;
            if (typeof data === 'string' || typeof data === 'number' || typeof data === 'boolean') return data;
            if (data instanceof Error) return {
                name: data.name,
                message: data.message,
                stack: data.stack
            };
            return JSON.parse(JSON.stringify(data));
        } catch (error) {
            return `[Serialization Error: ${error.message}]`;
        }
    }
}

/**
 * Abstract base class for log outputs
 */
export class LogOutput {
    constructor(options = {}) {
        this.options = {
            minLevel: LogLevel.INFO,
            formatter: null,
            ...options
        };
    }

    shouldLog(level) {
        return level >= this.options.minLevel;
    }

    format(entry) {
        if (this.options.formatter) {
            return this.options.formatter(entry);
        }
        return this.defaultFormat(entry);
    }

    defaultFormat(entry) {
        const timestamp = entry.timestamp.toISOString();
        const level = entry.levelName.padEnd(5);
        let formatted = `[${timestamp}] ${level} ${entry.message}`;

        if (entry.data !== null && entry.data !== undefined) {
            formatted += ` ${JSON.stringify(entry.data)}`;
        }

        return formatted;
    }

    async write(entry) {
        throw new Error('write method must be implemented by subclasses');
    }
}

/**
 * Console output for logs
 */
export class ConsoleOutput extends LogOutput {
    constructor(options = {}) {
        super({
            colors: true,
            ...options
        });

        this.colors = {
            [LogLevel.DEBUG]: '\x1b[36m',  // Cyan
            [LogLevel.INFO]: '\x1b[32m',   // Green
            [LogLevel.WARN]: '\x1b[33m',   // Yellow
            [LogLevel.ERROR]: '\x1b[31m'   // Red
        };

        this.resetColor = '\x1b[0m';
    }

    defaultFormat(entry) {
        const timestamp = entry.timestamp.toLocaleTimeString();
        const level = entry.levelName.padEnd(5);

        let formatted = `[${timestamp}] ${level} ${entry.message}`;

        if (entry.data !== null && entry.data !== undefined) {
            if (typeof entry.data === 'object') {
                formatted += '\n' + JSON.stringify(entry.data, null, 2);
            } else {
                formatted += ` ${entry.data}`;
            }
        }

        if (this.options.colors && typeof window === 'undefined') {
            const color = this.colors[entry.level] || '';
            formatted = `${color}${formatted}${this.resetColor}`;
        }

        return formatted;
    }

    async write(entry) {
        if (!this.shouldLog(entry.level)) return;

        const formatted = this.format(entry);

        // Use appropriate console method based on level
        switch (entry.level) {
            case LogLevel.DEBUG:
                console.debug(formatted);
                break;
            case LogLevel.INFO:
                console.info(formatted);
                break;
            case LogLevel.WARN:
                console.warn(formatted);
                if (entry.data instanceof Error) {
                    console.warn(entry.data);
                }
                break;
            case LogLevel.ERROR:
                console.error(formatted);
                if (entry.data instanceof Error) {
                    console.error(entry.data);
                }
                break;
        }
    }
}

/**
 * Memory buffer output for logs
 */
export class BufferOutput extends LogOutput {
    constructor(options = {}) {
        super({
            maxSize: 1000,
            ...options
        });

        this.buffer = [];
    }

    async write(entry) {
        if (!this.shouldLog(entry.level)) return;

        this.buffer.push(entry);

        // Maintain buffer size
        if (this.buffer.length > this.options.maxSize) {
            this.buffer.shift();
        }
    }

    getEntries(level = null, limit = null) {
        let entries = this.buffer;

        if (level !== null) {
            entries = entries.filter(entry => entry.level === level);
        }

        if (limit !== null) {
            entries = entries.slice(-limit);
        }

        return entries;
    }

    clear() {
        this.buffer = [];
    }

    getStats() {
        const stats = {
            total: this.buffer.length,
            byLevel: {}
        };

        for (const levelName of Object.values(LogLevelNames)) {
            stats.byLevel[levelName] = 0;
        }

        this.buffer.forEach(entry => {
            stats.byLevel[entry.levelName]++;
        });

        return stats;
    }
}

/**
 * Remote/HTTP output for logs
 */
export class RemoteOutput extends LogOutput {
    constructor(endpoint, options = {}) {
        super({
            batchSize: 10,
            batchTimeout: 5000,
            headers: {
                'Content-Type': 'application/json'
            },
            ...options
        });

        this.endpoint = endpoint;
        this.pendingEntries = [];
        this.batchTimer = null;
    }

    async write(entry) {
        if (!this.shouldLog(entry.level)) return;

        this.pendingEntries.push(entry);

        if (this.pendingEntries.length >= this.options.batchSize) {
            await this.flush();
        } else if (!this.batchTimer) {
            this.batchTimer = setTimeout(() => {
                this.flush();
            }, this.options.batchTimeout);
        }
    }

    async flush() {
        if (this.pendingEntries.length === 0) return;

        const entries = this.pendingEntries.splice(0);

        if (this.batchTimer) {
            clearTimeout(this.batchTimer);
            this.batchTimer = null;
        }

        try {
            const response = await fetch(this.endpoint, {
                method: 'POST',
                headers: this.options.headers,
                body: JSON.stringify({
                    logs: entries.map(entry => entry.toJSON())
                })
            });

            if (!response.ok) {
                console.error(`Failed to send logs: ${response.status} ${response.statusText}`);
            }
        } catch (error) {
            console.error('Error sending logs to remote endpoint:', error);
            // Could implement retry logic here
        }
    }
}

/**
 * Performance timer utility
 */
export class PerformanceTimer {
    constructor(name) {
        this.name = name;
        this.startTime = performance?.now() || Date.now();
        this.marks = {};
    }

    mark(name) {
        this.marks[name] = (performance?.now() || Date.now()) - this.startTime;
    }

    stop() {
        const endTime = performance?.now() || Date.now();
        const duration = endTime - this.startTime;

        return {
            name: this.name,
            duration,
            marks: { ...this.marks }
        };
    }
}

/**
 * Main Logger class
 */
export class Logger {
    constructor(options = {}) {
        this.options = {
            level: LogLevel.INFO,
            context: {},
            outputs: [],
            enablePerformanceTracking: true,
            ...options
        };

        // Default console output if none specified
        if (this.options.outputs.length === 0) {
            this.options.outputs.push(new ConsoleOutput({
                minLevel: this.options.level
            }));
        }

        this.context = { ...this.options.context };
        this.activeTimers = new Map();
    }

    setLevel(level) {
        this.options.level = level;

        // Update all outputs to use new level if they don't have their own
        this.options.outputs.forEach(output => {
            if (!output.options.hasOwnProperty('minLevel')) {
                output.options.minLevel = level;
            }
        });
    }

    addOutput(output) {
        this.options.outputs.push(output);
    }

    removeOutput(output) {
        const index = this.options.outputs.indexOf(output);
        if (index > -1) {
            this.options.outputs.splice(index, 1);
        }
    }

    setContext(context) {
        this.context = { ...this.context, ...context };
    }

    clearContext() {
        this.context = {};
    }

    async log(level, message, data = null, additionalContext = {}) {
        if (level < this.options.level) return;

        const entry = new LogEntry(level, message, data, {
            ...this.context,
            ...additionalContext
        });

        // Write to all outputs
        const writePromises = this.options.outputs.map(output =>
            output.write(entry).catch(error =>
                console.error('Error writing to log output:', error)
            )
        );

        await Promise.all(writePromises);
    }

    debug(message, data = null, context = {}) {
        return this.log(LogLevel.DEBUG, message, data, context);
    }

    info(message, data = null, context = {}) {
        return this.log(LogLevel.INFO, message, data, context);
    }

    warn(message, data = null, context = {}) {
        return this.log(LogLevel.WARN, message, data, context);
    }

    error(message, data = null, context = {}) {
        return this.log(LogLevel.ERROR, message, data, context);
    }

    // Performance monitoring methods
    time(name) {
        if (!this.options.enablePerformanceTracking) return null;

        const timer = new PerformanceTimer(name);
        this.activeTimers.set(name, timer);
        return timer;
    }

    timeEnd(name) {
        if (!this.options.enablePerformanceTracking) return null;

        const timer = this.activeTimers.get(name);
        if (!timer) {
            this.warn(`Timer '${name}' not found`);
            return null;
        }

        const result = timer.stop();
        this.activeTimers.delete(name);

        this.info(`Performance: ${name}`, {
            duration: `${result.duration.toFixed(2)}ms`,
            marks: result.marks
        });

        return result;
    }

    // Utility methods for common logging patterns
    async logApiRequest(method, url, duration, status) {
        const level = status >= 400 ? LogLevel.ERROR : LogLevel.INFO;
        await this.log(level, `API Request: ${method} ${url}`, {
            method,
            url,
            duration: `${duration}ms`,
            status
        });
    }

    async logUserAction(action, userId, details = {}) {
        await this.info(`User Action: ${action}`, {
            action,
            userId,
            ...details
        });
    }

    async logError(error, context = {}) {
        const errorData = {
            name: error.name,
            message: error.message,
            stack: error.stack,
            ...context
        };

        await this.error('Error occurred', errorData);
    }

    // Batch logging for high-volume scenarios
    createBatchLogger(flushInterval = 1000) {
        const pendingLogs = [];
        let flushTimer = null;

        const flush = async () => {
            if (pendingLogs.length === 0) return;

            const logs = pendingLogs.splice(0);
            const promises = logs.map(({ level, message, data, context }) =>
                this.log(level, message, data, context)
            );

            await Promise.all(promises);

            if (flushTimer) {
                clearTimeout(flushTimer);
                flushTimer = null;
            }
        };

        const batchLog = (level, message, data = null, context = {}) => {
            pendingLogs.push({ level, message, data, context });

            if (!flushTimer) {
                flushTimer = setTimeout(flush, flushInterval);
            }
        };

        return {
            debug: (msg, data, ctx) => batchLog(LogLevel.DEBUG, msg, data, ctx),
            info: (msg, data, ctx) => batchLog(LogLevel.INFO, msg, data, ctx),
            warn: (msg, data, ctx) => batchLog(LogLevel.WARN, msg, data, ctx),
            error: (msg, data, ctx) => batchLog(LogLevel.ERROR, msg, data, ctx),
            flush
        };
    }

    // Child logger with inherited context
    createChild(childContext = {}) {
        return new Logger({
            ...this.options,
            context: { ...this.context, ...childContext }
        });
    }
}

// Factory functions
export function createLogger(options = {}) {
    return new Logger(options);
}

export function createConsoleLogger(level = LogLevel.INFO) {
    return new Logger({
        level,
        outputs: [new ConsoleOutput({ minLevel: level })]
    });
}

export function createBufferedLogger(maxSize = 1000) {
    return new Logger({
        outputs: [
            new ConsoleOutput(),
            new BufferOutput({ maxSize })
        ]
    });
}

// Global logger instance
export const logger = createConsoleLogger();

// Export all classes and utilities
export {
    Logger,
    LogEntry,
    LogOutput,
    ConsoleOutput,
    BufferOutput,
    RemoteOutput,
    PerformanceTimer,
    LogLevel,
    LogLevelNames
};

export default Logger;