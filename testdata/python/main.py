#!/usr/bin/env python3
"""
Main application module for the Python test project.

This module demonstrates various Python features including:
- Class definitions with inheritance
- Decorators and property methods
- Context managers
- Exception handling
- Type hints and dataclasses
"""

import asyncio
import logging
import os
import sys
from datetime import datetime, timedelta
from typing import Dict, List, Optional, Union, Any
from dataclasses import dataclass, field
from contextlib import contextmanager
from pathlib import Path

logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s',
    handlers=[
        logging.StreamHandler(sys.stdout),
        logging.FileHandler('app.log')
    ]
)

logger = logging.getLogger(__name__)


@dataclass
class AppConfig:
    """Application configuration dataclass."""
    debug: bool = False
    host: str = "localhost"
    port: int = 8080
    database_url: str = "sqlite:///app.db"
    secret_key: str = field(default_factory=lambda: os.urandom(32).hex())
    allowed_origins: List[str] = field(default_factory=lambda: ["*"])
    rate_limit: int = 100
    timeout: float = 30.0

    def __post_init__(self):
        """Validate configuration after initialization."""
        if self.port < 1 or self.port > 65535:
            raise ValueError("Port must be between 1 and 65535")
        if self.rate_limit < 1:
            raise ValueError("Rate limit must be positive")


class ConfigurationError(Exception):
    """Custom exception for configuration errors."""
    pass


class ValidationError(Exception):
    """Custom exception for validation errors."""
    pass


class BaseService:
    """Base service class providing common functionality."""

    def __init__(self, config: AppConfig):
        self.config = config
        self.logger = logging.getLogger(self.__class__.__name__)
        self._initialized = False

    async def initialize(self) -> None:
        """Initialize the service."""
        if self._initialized:
            return

        self.logger.info("Initializing %s", self.__class__.__name__)
        await self._setup()
        self._initialized = True

    async def _setup(self) -> None:
        """Override this method in subclasses for custom setup."""
        pass

    async def cleanup(self) -> None:
        """Cleanup resources."""
        if not self._initialized:
            return

        self.logger.info("Cleaning up %s", self.__class__.__name__)
        await self._teardown()
        self._initialized = False

    async def _teardown(self) -> None:
        """Override this method in subclasses for custom teardown."""
        pass

    def is_initialized(self) -> bool:
        """Check if the service is initialized."""
        return self._initialized


class DatabaseService(BaseService):
    """Database service for handling data operations."""

    def __init__(self, config: AppConfig):
        super().__init__(config)
        self._connection_pool = None
        self._transaction_count = 0

    async def _setup(self) -> None:
        """Setup database connection pool."""
        self.logger.info("Setting up database connection to %s", self.config.database_url)
        # Simulated database connection setup
        self._connection_pool = f"Connection pool for {self.config.database_url}"

    async def _teardown(self) -> None:
        """Teardown database connections."""
        if self._connection_pool:
            self.logger.info("Closing database connections")
            self._connection_pool = None

    @contextmanager
    def transaction(self):
        """Database transaction context manager."""
        self._transaction_count += 1
        transaction_id = self._transaction_count

        try:
            self.logger.debug("Starting transaction %d", transaction_id)
            yield transaction_id
            self.logger.debug("Committing transaction %d", transaction_id)
        except Exception as e:
            self.logger.error("Rolling back transaction %d: %s", transaction_id, e)
            raise
        finally:
            self.logger.debug("Ending transaction %d", transaction_id)

    async def execute_query(self, query: str, params: Optional[Dict] = None) -> List[Dict[str, Any]]:
        """Execute a database query."""
        if not self._initialized:
            raise RuntimeError("Database service not initialized")

        self.logger.debug("Executing query: %s", query)

        # Simulate query execution
        await asyncio.sleep(0.1)  # Simulate network delay

        # Return mock data based on query type
        if query.lower().startswith('select'):
            return [{"id": 1, "name": "test", "created_at": datetime.now().isoformat()}]
        return []

    async def health_check(self) -> bool:
        """Check database health."""
        try:
            await self.execute_query("SELECT 1")
            return True
        except Exception as e:
            self.logger.error("Database health check failed: %s", e)
            return False


class CacheService(BaseService):
    """In-memory cache service."""

    def __init__(self, config: AppConfig, ttl_seconds: int = 300):
        super().__init__(config)
        self._cache: Dict[str, Dict[str, Any]] = {}
        self._ttl_seconds = ttl_seconds

    def _is_expired(self, item: Dict[str, Any]) -> bool:
        """Check if a cache item is expired."""
        expiry = item.get('expiry')
        if not expiry:
            return False
        return datetime.now() > expiry

    def get(self, key: str) -> Optional[Any]:
        """Get a value from cache."""
        if key not in self._cache:
            return None

        item = self._cache[key]
        if self._is_expired(item):
            del self._cache[key]
            return None

        return item['value']

    def set(self, key: str, value: Any, ttl: Optional[int] = None) -> None:
        """Set a value in cache."""
        ttl = ttl or self._ttl_seconds
        expiry = datetime.now() + timedelta(seconds=ttl)

        self._cache[key] = {
            'value': value,
            'expiry': expiry,
            'created_at': datetime.now()
        }

        self.logger.debug("Cached item with key: %s", key)

    def delete(self, key: str) -> bool:
        """Delete a value from cache."""
        if key in self._cache:
            del self._cache[key]
            self.logger.debug("Deleted cache key: %s", key)
            return True
        return False

    def clear(self) -> None:
        """Clear all cache entries."""
        self._cache.clear()
        self.logger.info("Cache cleared")

    def cleanup_expired(self) -> int:
        """Remove expired items from cache."""
        expired_keys = [
            key for key, item in self._cache.items()
            if self._is_expired(item)
        ]

        for key in expired_keys:
            del self._cache[key]

        self.logger.info("Cleaned up %d expired cache entries", len(expired_keys))
        return len(expired_keys)


class APIService(BaseService):
    """API service for handling HTTP requests."""

    def __init__(self, config: AppConfig, db_service: DatabaseService, cache_service: CacheService):
        super().__init__(config)
        self.db_service = db_service
        self.cache_service = cache_service
        self._routes: Dict[str, callable] = {}
        self._middleware: List[callable] = []

    def route(self, path: str, methods: List[str] = None):
        """Decorator for registering routes."""
        methods = methods or ['GET']

        def decorator(func):
            self._routes[f"{','.join(methods)}:{path}"] = func
            return func

        return decorator

    def middleware(self, func):
        """Decorator for registering middleware."""
        self._middleware.append(func)
        return func

    async def handle_request(self, method: str, path: str, data: Dict = None) -> Dict[str, Any]:
        """Handle an incoming request."""
        route_key = f"{method}:{path}"

        if route_key not in self._routes:
            return {"error": "Route not found", "status": 404}

        # Apply middleware
        for middleware_func in self._middleware:
            try:
                await middleware_func(method, path, data)
            except Exception as e:
                return {"error": f"Middleware error: {str(e)}", "status": 500}

        # Execute route handler
        try:
            handler = self._routes[route_key]
            result = await handler(data or {})
            return {"data": result, "status": 200}
        except Exception as e:
            self.logger.error("Error handling request %s %s: %s", method, path, e)
            return {"error": str(e), "status": 500}


class Application:
    """Main application class orchestrating all services."""

    def __init__(self, config_path: Optional[str] = None):
        self.config = self._load_config(config_path)
        self.logger = logging.getLogger(self.__class__.__name__)

        # Initialize services
        self.db_service = DatabaseService(self.config)
        self.cache_service = CacheService(self.config)
        self.api_service = APIService(self.config, self.db_service, self.cache_service)

        self._setup_routes()
        self._setup_middleware()

    def _load_config(self, config_path: Optional[str]) -> AppConfig:
        """Load application configuration."""
        if config_path and Path(config_path).exists():
            # In a real app, would load from file
            self.logger.info("Loading config from %s", config_path)

        # Load from environment variables
        return AppConfig(
            debug=os.getenv('DEBUG', 'false').lower() == 'true',
            host=os.getenv('HOST', 'localhost'),
            port=int(os.getenv('PORT', '8080')),
            database_url=os.getenv('DATABASE_URL', 'sqlite:///app.db'),
            rate_limit=int(os.getenv('RATE_LIMIT', '100'))
        )

    def _setup_routes(self):
        """Setup API routes."""

        @self.api_service.route('/health')
        async def health_check(data: Dict) -> Dict[str, Any]:
            """Health check endpoint."""
            db_healthy = await self.db_service.health_check()
            return {
                "status": "healthy" if db_healthy else "unhealthy",
                "database": db_healthy,
                "timestamp": datetime.now().isoformat()
            }

        @self.api_service.route('/users', ['GET'])
        async def get_users(data: Dict) -> List[Dict[str, Any]]:
            """Get users endpoint."""
            cached_users = self.cache_service.get('users')
            if cached_users:
                return cached_users

            users = await self.db_service.execute_query("SELECT * FROM users")
            self.cache_service.set('users', users, ttl=60)
            return users

        @self.api_service.route('/users', ['POST'])
        async def create_user(data: Dict) -> Dict[str, Any]:
            """Create user endpoint."""
            if not data.get('name'):
                raise ValidationError("Name is required")

            with self.db_service.transaction():
                result = await self.db_service.execute_query(
                    "INSERT INTO users (name) VALUES (?)",
                    {"name": data["name"]}
                )

            self.cache_service.delete('users')

            return {"message": "User created", "id": 1}

    def _setup_middleware(self):
        """Setup API middleware."""

        @self.api_service.middleware
        async def logging_middleware(method: str, path: str, data: Dict):
            """Log all requests."""
            self.logger.info("Request: %s %s", method, path)

        @self.api_service.middleware
        async def rate_limit_middleware(method: str, path: str, data: Dict):
            """Simple rate limiting."""
            # In a real app, would implement proper rate limiting
            pass

    async def start(self) -> None:
        """Start the application."""
        self.logger.info("Starting application")

        try:
            await self.db_service.initialize()
            await self.cache_service.initialize()
            await self.api_service.initialize()

            self.logger.info("Application started successfully on %s:%d",
                           self.config.host, self.config.port)

        except Exception as e:
            self.logger.error("Failed to start application: %s", e)
            await self.stop()
            raise

    async def stop(self) -> None:
        """Stop the application."""
        self.logger.info("Stopping application")

        await self.api_service.cleanup()
        await self.cache_service.cleanup()
        await self.db_service.cleanup()

        self.logger.info("Application stopped")

    async def run_server(self):
        """Run the application server (simplified)."""
        await self.start()

        try:
            # Simulate server running
            while True:
                await asyncio.sleep(1)
        except KeyboardInterrupt:
            self.logger.info("Received shutdown signal")
        finally:
            await self.stop()


async def main():
    """Main entry point."""
    app = Application()

    try:
        await app.start()

        health_result = await app.api_service.handle_request('GET', '/health')
        print(f"Health check: {health_result}")

        users_result = await app.api_service.handle_request('GET', '/users')
        print(f"Users: {users_result}")

        create_result = await app.api_service.handle_request(
            'POST', '/users', {'name': 'John Doe'}
        )
        print(f"Create user: {create_result}")

    except Exception as e:
        logger.error("Application error: %s", e)
        sys.exit(1)
    finally:
        await app.stop()


if __name__ == "__main__":
    asyncio.run(main())