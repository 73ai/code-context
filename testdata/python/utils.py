"""
Utility functions and helper classes for the Python test project.

This module provides:
- String manipulation utilities
- Data validation functions
- Caching decorators
- Date/time helpers
- File I/O utilities
- Encryption/hashing functions
- Mathematical utilities
- Collection helpers
"""

import hashlib
import hmac
import json
import os
import re
import secrets
import time
import uuid
from collections import Counter, defaultdict, deque
from contextlib import contextmanager
from datetime import datetime, timedelta, timezone
from decimal import Decimal, InvalidOperation
from functools import wraps, lru_cache
from pathlib import Path
from typing import (
    Any, Callable, Dict, List, Optional, Union, Tuple, Set,
    Generator, Iterator, TypeVar, Generic, Protocol
)
from urllib.parse import urlparse, parse_qs


# Type variables for generic functions
T = TypeVar('T')
K = TypeVar('K')
V = TypeVar('V')


class StringUtils:
    """Collection of string manipulation utilities."""

    @staticmethod
    def slugify(text: str, max_length: int = 50) -> str:
        """Convert text to URL-friendly slug."""
        # Convert to lowercase and replace spaces with hyphens
        slug = re.sub(r'[^\w\s-]', '', text.lower())
        slug = re.sub(r'[-\s]+', '-', slug).strip('-')
        return slug[:max_length]

    @staticmethod
    def truncate(text: str, max_length: int, suffix: str = "...") -> str:
        """Truncate text to specified length with optional suffix."""
        if len(text) <= max_length:
            return text
        return text[:max_length - len(suffix)] + suffix

    @staticmethod
    def camel_to_snake(text: str) -> str:
        """Convert camelCase to snake_case."""
        # Insert underscore before uppercase letters
        s1 = re.sub('(.)([A-Z][a-z]+)', r'\1_\2', text)
        return re.sub('([a-z0-9])([A-Z])', r'\1_\2', s1).lower()

    @staticmethod
    def snake_to_camel(text: str) -> str:
        """Convert snake_case to camelCase."""
        components = text.split('_')
        return components[0] + ''.join(word.capitalize() for word in components[1:])

    @staticmethod
    def is_valid_email(email: str) -> bool:
        """Validate email address format."""
        pattern = r'^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$'
        return bool(re.match(pattern, email))

    @staticmethod
    def extract_domain(email: str) -> Optional[str]:
        """Extract domain from email address."""
        if '@' in email:
            return email.split('@')[1].lower()
        return None

    @staticmethod
    def mask_string(text: str, visible_chars: int = 4, mask_char: str = '*') -> str:
        """Mask string showing only specified number of characters."""
        if len(text) <= visible_chars * 2:
            return mask_char * len(text)

        visible_start = text[:visible_chars]
        visible_end = text[-visible_chars:]
        masked_middle = mask_char * (len(text) - visible_chars * 2)
        return visible_start + masked_middle + visible_end

    @staticmethod
    def extract_urls(text: str) -> List[str]:
        """Extract URLs from text."""
        url_pattern = r'http[s]?://(?:[a-zA-Z]|[0-9]|[$-_@.&+]|[!*\\(\\),]|(?:%[0-9a-fA-F][0-9a-fA-F]))+'
        return re.findall(url_pattern, text)

    @staticmethod
    def generate_password(
        length: int = 12,
        use_uppercase: bool = True,
        use_lowercase: bool = True,
        use_digits: bool = True,
        use_symbols: bool = True
    ) -> str:
        """Generate a secure random password."""
        chars = ""
        if use_lowercase:
            chars += "abcdefghijklmnopqrstuvwxyz"
        if use_uppercase:
            chars += "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
        if use_digits:
            chars += "0123456789"
        if use_symbols:
            chars += "!@#$%^&*()_+-=[]{}|;:,.<>?"

        if not chars:
            raise ValueError("At least one character type must be enabled")

        return ''.join(secrets.choice(chars) for _ in range(length))


class ValidationUtils:
    """Data validation utilities."""

    @staticmethod
    def is_valid_uuid(uuid_string: str) -> bool:
        """Check if string is a valid UUID."""
        try:
            uuid.UUID(uuid_string)
            return True
        except ValueError:
            return False

    @staticmethod
    def is_valid_phone(phone: str, country_code: str = "US") -> bool:
        """Basic phone number validation (simplified)."""
        # Remove all non-digit characters
        digits_only = re.sub(r'\D', '', phone)

        if country_code == "US":
            return len(digits_only) == 10 or (len(digits_only) == 11 and digits_only[0] == '1')

        # Basic international validation (7-15 digits)
        return 7 <= len(digits_only) <= 15

    @staticmethod
    def validate_credit_card(card_number: str) -> Tuple[bool, Optional[str]]:
        """Validate credit card using Luhn algorithm and detect card type."""
        # Remove spaces and hyphens
        card_number = re.sub(r'[\s-]', '', card_number)

        if not card_number.isdigit():
            return False, None

        # Luhn algorithm
        def luhn_checksum(card_num):
            def digits_of(number):
                return [int(d) for d in str(number)]

            digits = digits_of(card_num)
            odd_digits = digits[-1::-2]
            even_digits = digits[-2::-2]
            checksum = sum(odd_digits)
            for digit in even_digits:
                checksum += sum(digits_of(digit * 2))
            return checksum % 10

        is_valid = luhn_checksum(card_number) == 0

        # Detect card type
        card_type = None
        if is_valid:
            if card_number.startswith('4'):
                card_type = 'Visa'
            elif card_number.startswith(('51', '52', '53', '54', '55')):
                card_type = 'MasterCard'
            elif card_number.startswith(('34', '37')):
                card_type = 'American Express'
            elif card_number.startswith('6011'):
                card_type = 'Discover'

        return is_valid, card_type

    @staticmethod
    def validate_password_strength(password: str) -> Dict[str, Any]:
        """Analyze password strength and return feedback."""
        feedback = {
            'score': 0,
            'length': len(password),
            'has_uppercase': bool(re.search(r'[A-Z]', password)),
            'has_lowercase': bool(re.search(r'[a-z]', password)),
            'has_digits': bool(re.search(r'\d', password)),
            'has_symbols': bool(re.search(r'[!@#$%^&*(),.?":{}|<>]', password)),
            'common_patterns': [],
            'suggestions': []
        }

        # Calculate score
        if feedback['length'] >= 8:
            feedback['score'] += 1
        if feedback['length'] >= 12:
            feedback['score'] += 1
        if feedback['has_uppercase']:
            feedback['score'] += 1
        if feedback['has_lowercase']:
            feedback['score'] += 1
        if feedback['has_digits']:
            feedback['score'] += 1
        if feedback['has_symbols']:
            feedback['score'] += 1

        # Check for common patterns
        if re.search(r'(.)\1{2,}', password):  # Repeated characters
            feedback['common_patterns'].append('repeated_characters')
        if re.search(r'(012|123|234|345|456|567|678|789)', password):
            feedback['common_patterns'].append('sequential_numbers')
        if re.search(r'(abc|bcd|cde|def|efg|fgh|ghi|hij|ijk|jkl|klm|lmn|mno|nop|opq|pqr|qrs|rst|stu|tuv|uvw|vwx|wxy|xyz)', password.lower()):
            feedback['common_patterns'].append('sequential_letters')

        # Generate suggestions
        if feedback['length'] < 8:
            feedback['suggestions'].append('Use at least 8 characters')
        if not feedback['has_uppercase']:
            feedback['suggestions'].append('Add uppercase letters')
        if not feedback['has_lowercase']:
            feedback['suggestions'].append('Add lowercase letters')
        if not feedback['has_digits']:
            feedback['suggestions'].append('Add numbers')
        if not feedback['has_symbols']:
            feedback['suggestions'].append('Add special characters')

        return feedback


class CacheUtils:
    """Caching utilities and decorators."""

    @staticmethod
    def memoize(func: Callable) -> Callable:
        """Simple memoization decorator."""
        cache = {}

        @wraps(func)
        def wrapper(*args, **kwargs):
            key = str(args) + str(sorted(kwargs.items()))
            if key not in cache:
                cache[key] = func(*args, **kwargs)
            return cache[key]

        wrapper.cache = cache
        wrapper.clear_cache = lambda: cache.clear()
        return wrapper

    @staticmethod
    def timed_cache(expiry_seconds: int = 300):
        """Caching decorator with expiration time."""
        def decorator(func: Callable) -> Callable:
            cache = {}

            @wraps(func)
            def wrapper(*args, **kwargs):
                key = str(args) + str(sorted(kwargs.items()))
                current_time = time.time()

                if key in cache:
                    value, timestamp = cache[key]
                    if current_time - timestamp < expiry_seconds:
                        return value

                result = func(*args, **kwargs)
                cache[key] = (result, current_time)
                return result

            wrapper.cache = cache
            wrapper.clear_cache = lambda: cache.clear()
            return wrapper

        return decorator


class DateTimeUtils:
    """Date and time manipulation utilities."""

    @staticmethod
    def parse_date_string(date_str: str) -> Optional[datetime]:
        """Parse date string in various formats."""
        formats = [
            '%Y-%m-%d',
            '%Y-%m-%d %H:%M:%S',
            '%Y-%m-%dT%H:%M:%S',
            '%Y-%m-%dT%H:%M:%SZ',
            '%Y-%m-%dT%H:%M:%S.%fZ',
            '%d/%m/%Y',
            '%m/%d/%Y',
            '%d-%m-%Y',
            '%m-%d-%Y'
        ]

        for fmt in formats:
            try:
                return datetime.strptime(date_str, fmt)
            except ValueError:
                continue

        return None

    @staticmethod
    def format_duration(seconds: Union[int, float]) -> str:
        """Format duration in seconds to human-readable string."""
        if seconds < 60:
            return f"{seconds:.1f}s"
        elif seconds < 3600:
            return f"{seconds/60:.1f}m"
        elif seconds < 86400:
            return f"{seconds/3600:.1f}h"
        else:
            return f"{seconds/86400:.1f}d"

    @staticmethod
    def get_age(birth_date: datetime) -> int:
        """Calculate age from birth date."""
        today = datetime.now().date()
        birth_date = birth_date.date() if isinstance(birth_date, datetime) else birth_date
        return today.year - birth_date.year - ((today.month, today.day) < (birth_date.month, birth_date.day))

    @staticmethod
    def get_business_days(start_date: datetime, end_date: datetime) -> int:
        """Count business days between two dates."""
        business_days = 0
        current_date = start_date.date()
        end_date = end_date.date()

        while current_date <= end_date:
            if current_date.weekday() < 5:  # Monday = 0, Friday = 4
                business_days += 1
            current_date += timedelta(days=1)

        return business_days

    @staticmethod
    def get_timezone_offset(tz_name: str = 'UTC') -> int:
        """Get timezone offset in hours from UTC."""
        # Simplified implementation - in real app would use pytz
        tz_offsets = {
            'UTC': 0,
            'EST': -5,
            'CST': -6,
            'MST': -7,
            'PST': -8,
            'GMT': 0,
            'CET': 1,
            'JST': 9
        }
        return tz_offsets.get(tz_name.upper(), 0)

    @staticmethod
    def is_weekend(date: datetime) -> bool:
        """Check if date falls on weekend."""
        return date.weekday() >= 5  # Saturday = 5, Sunday = 6


class FileUtils:
    """File and path manipulation utilities."""

    @staticmethod
    def ensure_directory(path: Union[str, Path]) -> Path:
        """Create directory if it doesn't exist."""
        path = Path(path)
        path.mkdir(parents=True, exist_ok=True)
        return path

    @staticmethod
    def get_file_size_str(size_bytes: int) -> str:
        """Convert file size in bytes to human-readable string."""
        for unit in ['B', 'KB', 'MB', 'GB', 'TB']:
            if size_bytes < 1024:
                return f"{size_bytes:.1f} {unit}"
            size_bytes /= 1024
        return f"{size_bytes:.1f} PB"

    @staticmethod
    def get_file_extension(filename: str) -> str:
        """Get file extension (without dot)."""
        return Path(filename).suffix.lstrip('.')

    @staticmethod
    def safe_filename(filename: str) -> str:
        """Convert filename to safe version by removing invalid characters."""
        # Remove or replace invalid characters
        filename = re.sub(r'[<>:"/\\|?*]', '_', filename)
        # Remove leading/trailing dots and spaces
        filename = filename.strip('. ')
        # Limit length
        if len(filename) > 255:
            name, ext = os.path.splitext(filename)
            filename = name[:255-len(ext)] + ext
        return filename

    @staticmethod
    def read_json_file(filepath: Union[str, Path]) -> Dict[str, Any]:
        """Read and parse JSON file."""
        with open(filepath, 'r', encoding='utf-8') as f:
            return json.load(f)

    @staticmethod
    def write_json_file(filepath: Union[str, Path], data: Dict[str, Any], indent: int = 2) -> None:
        """Write data to JSON file."""
        with open(filepath, 'w', encoding='utf-8') as f:
            json.dump(data, f, indent=indent, ensure_ascii=False)

    @contextmanager
    def temporary_file(suffix: str = '', prefix: str = 'tmp'):
        """Context manager for temporary file."""
        import tempfile
        temp_file = None
        try:
            temp_file = tempfile.NamedTemporaryFile(delete=False, suffix=suffix, prefix=prefix)
            yield temp_file.name
        finally:
            if temp_file and os.path.exists(temp_file.name):
                os.unlink(temp_file.name)


class CryptoUtils:
    """Cryptography and hashing utilities."""

    @staticmethod
    def hash_password(password: str, salt: Optional[str] = None) -> Tuple[str, str]:
        """Hash password with salt using SHA-256."""
        if salt is None:
            salt = secrets.token_hex(32)

        # Combine password and salt
        password_salt = (password + salt).encode('utf-8')
        password_hash = hashlib.sha256(password_salt).hexdigest()

        return password_hash, salt

    @staticmethod
    def verify_password(password: str, password_hash: str, salt: str) -> bool:
        """Verify password against hash and salt."""
        computed_hash, _ = CryptoUtils.hash_password(password, salt)
        return hmac.compare_digest(password_hash, computed_hash)

    @staticmethod
    def generate_token(length: int = 32) -> str:
        """Generate secure random token."""
        return secrets.token_urlsafe(length)

    @staticmethod
    def generate_api_key() -> str:
        """Generate API key with specific format."""
        prefix = "ak"
        random_part = secrets.token_hex(16)
        checksum = hashlib.md5(random_part.encode()).hexdigest()[:4]
        return f"{prefix}_{random_part}_{checksum}"

    @staticmethod
    def hash_file(filepath: Union[str, Path], algorithm: str = 'sha256') -> str:
        """Calculate hash of file contents."""
        hash_algo = hashlib.new(algorithm)

        with open(filepath, 'rb') as f:
            for chunk in iter(lambda: f.read(4096), b""):
                hash_algo.update(chunk)

        return hash_algo.hexdigest()

    @staticmethod
    def simple_encrypt(text: str, key: str) -> str:
        """Simple XOR encryption (for demonstration only, not secure)."""
        key_bytes = key.encode('utf-8')
        text_bytes = text.encode('utf-8')

        encrypted = bytearray()
        for i, byte in enumerate(text_bytes):
            encrypted.append(byte ^ key_bytes[i % len(key_bytes)])

        return encrypted.hex()

    @staticmethod
    def simple_decrypt(encrypted_hex: str, key: str) -> str:
        """Simple XOR decryption."""
        key_bytes = key.encode('utf-8')
        encrypted_bytes = bytes.fromhex(encrypted_hex)

        decrypted = bytearray()
        for i, byte in enumerate(encrypted_bytes):
            decrypted.append(byte ^ key_bytes[i % len(key_bytes)])

        return decrypted.decode('utf-8')


class MathUtils:
    """Mathematical utilities and calculations."""

    @staticmethod
    def safe_divide(numerator: Union[int, float], denominator: Union[int, float], default: Union[int, float] = 0) -> Union[int, float]:
        """Safe division that handles division by zero."""
        try:
            return numerator / denominator
        except ZeroDivisionError:
            return default

    @staticmethod
    def percentage(part: Union[int, float], total: Union[int, float]) -> float:
        """Calculate percentage with safe division."""
        return MathUtils.safe_divide(part * 100, total, 0.0)

    @staticmethod
    def round_decimal(value: Union[int, float, Decimal], places: int = 2) -> Decimal:
        """Round value to specified decimal places."""
        try:
            if isinstance(value, Decimal):
                return value.quantize(Decimal('0.' + '0' * places))
            else:
                return Decimal(str(value)).quantize(Decimal('0.' + '0' * places))
        except InvalidOperation:
            return Decimal('0')

    @staticmethod
    def clamp(value: Union[int, float], min_val: Union[int, float], max_val: Union[int, float]) -> Union[int, float]:
        """Clamp value between min and max."""
        return max(min_val, min(max_val, value))

    @staticmethod
    def average(values: List[Union[int, float]]) -> float:
        """Calculate average of list of numbers."""
        if not values:
            return 0.0
        return sum(values) / len(values)

    @staticmethod
    def median(values: List[Union[int, float]]) -> float:
        """Calculate median of list of numbers."""
        if not values:
            return 0.0

        sorted_values = sorted(values)
        n = len(sorted_values)

        if n % 2 == 0:
            return (sorted_values[n // 2 - 1] + sorted_values[n // 2]) / 2
        else:
            return sorted_values[n // 2]

    @staticmethod
    def compound_interest(principal: float, rate: float, time: float, compound_frequency: int = 1) -> float:
        """Calculate compound interest."""
        return principal * (1 + rate / compound_frequency) ** (compound_frequency * time)


class CollectionUtils:
    """Collection manipulation utilities."""

    @staticmethod
    def chunk_list(lst: List[T], chunk_size: int) -> Generator[List[T], None, None]:
        """Split list into chunks of specified size."""
        for i in range(0, len(lst), chunk_size):
            yield lst[i:i + chunk_size]

    @staticmethod
    def flatten_list(nested_list: List[List[T]]) -> List[T]:
        """Flatten nested list structure."""
        return [item for sublist in nested_list for item in sublist]

    @staticmethod
    def remove_duplicates(lst: List[T]) -> List[T]:
        """Remove duplicates while preserving order."""
        seen = set()
        result = []
        for item in lst:
            if item not in seen:
                seen.add(item)
                result.append(item)
        return result

    @staticmethod
    def group_by(lst: List[T], key_func: Callable[[T], K]) -> Dict[K, List[T]]:
        """Group list items by key function."""
        groups = defaultdict(list)
        for item in lst:
            key = key_func(item)
            groups[key].append(item)
        return dict(groups)

    @staticmethod
    def find_duplicates(lst: List[T]) -> List[T]:
        """Find duplicate items in list."""
        counts = Counter(lst)
        return [item for item, count in counts.items() if count > 1]

    @staticmethod
    def deep_merge_dicts(dict1: Dict, dict2: Dict) -> Dict:
        """Deep merge two dictionaries."""
        result = dict1.copy()

        for key, value in dict2.items():
            if key in result and isinstance(result[key], dict) and isinstance(value, dict):
                result[key] = CollectionUtils.deep_merge_dicts(result[key], value)
            else:
                result[key] = value

        return result

    @staticmethod
    def paginate(lst: List[T], page: int, per_page: int) -> Tuple[List[T], Dict[str, Any]]:
        """Paginate list and return items with pagination info."""
        total = len(lst)
        total_pages = (total + per_page - 1) // per_page

        start = (page - 1) * per_page
        end = start + per_page
        items = lst[start:end]

        pagination_info = {
            'page': page,
            'per_page': per_page,
            'total': total,
            'total_pages': total_pages,
            'has_prev': page > 1,
            'has_next': page < total_pages
        }

        return items, pagination_info


class URLUtils:
    """URL manipulation utilities."""

    @staticmethod
    def parse_url(url: str) -> Dict[str, Any]:
        """Parse URL into components."""
        parsed = urlparse(url)
        query_params = parse_qs(parsed.query)

        # Convert single-item lists to strings
        for key, value in query_params.items():
            if len(value) == 1:
                query_params[key] = value[0]

        return {
            'scheme': parsed.scheme,
            'netloc': parsed.netloc,
            'path': parsed.path,
            'params': parsed.params,
            'query': query_params,
            'fragment': parsed.fragment,
            'hostname': parsed.hostname,
            'port': parsed.port
        }

    @staticmethod
    def build_url(base_url: str, path: str = '', params: Dict[str, Any] = None) -> str:
        """Build URL from components."""
        from urllib.parse import urljoin, urlencode

        url = urljoin(base_url.rstrip('/') + '/', path.lstrip('/'))

        if params:
            query_string = urlencode(params)
            separator = '&' if '?' in url else '?'
            url = f"{url}{separator}{query_string}"

        return url

    @staticmethod
    def is_valid_url(url: str) -> bool:
        """Check if string is a valid URL."""
        try:
            parsed = urlparse(url)
            return bool(parsed.netloc and parsed.scheme)
        except Exception:
            return False


# Global utility instances for convenience
string_utils = StringUtils()
validation_utils = ValidationUtils()
cache_utils = CacheUtils()
datetime_utils = DateTimeUtils()
file_utils = FileUtils()
crypto_utils = CryptoUtils()
math_utils = MathUtils()
collection_utils = CollectionUtils()
url_utils = URLUtils()


# Convenience functions
def slugify(text: str) -> str:
    """Convenience function for string slugification."""
    return string_utils.slugify(text)


def validate_email(email: str) -> bool:
    """Convenience function for email validation."""
    return string_utils.is_valid_email(email)


@cache_utils.timed_cache(300)  # 5-minute cache
def expensive_computation(n: int) -> int:
    """Example of cached expensive computation."""
    # Simulate expensive computation
    result = 0
    for i in range(n):
        result += i ** 2
    return result


def format_currency(amount: Union[int, float, Decimal], currency: str = 'USD') -> str:
    """Format amount as currency string."""
    if isinstance(amount, (int, float)):
        amount = Decimal(str(amount))

    symbols = {
        'USD': '$',
        'EUR': '€',
        'GBP': '£',
        'JPY': '¥'
    }

    symbol = symbols.get(currency.upper(), currency)
    return f"{symbol}{amount:,.2f}"


def retry(max_attempts: int = 3, delay: float = 1.0, exponential_backoff: bool = True):
    """Decorator for retrying failed function calls."""
    def decorator(func: Callable) -> Callable:
        @wraps(func)
        def wrapper(*args, **kwargs):
            last_exception = None

            for attempt in range(max_attempts):
                try:
                    return func(*args, **kwargs)
                except Exception as e:
                    last_exception = e
                    if attempt < max_attempts - 1:
                        wait_time = delay * (2 ** attempt if exponential_backoff else 1)
                        time.sleep(wait_time)

            raise last_exception

        return wrapper
    return decorator


# Example usage and testing functions
if __name__ == "__main__":
    # Test various utilities
    print("Testing string utilities...")
    print(f"Slug: {slugify('Hello World! 123')}")
    print(f"Email valid: {validate_email('test@example.com')}")

    print("\nTesting math utilities...")
    print(f"Safe divide: {math_utils.safe_divide(10, 0, 'N/A')}")
    print(f"Percentage: {math_utils.percentage(25, 100)}")

    print("\nTesting collection utilities...")
    test_list = [1, 2, 3, 4, 5, 6, 7, 8, 9]
    chunks = list(collection_utils.chunk_list(test_list, 3))
    print(f"Chunks: {chunks}")

    print("\nTesting cached function...")
    start_time = time.time()
    result1 = expensive_computation(1000)
    first_call_time = time.time() - start_time

    start_time = time.time()
    result2 = expensive_computation(1000)  # Should be cached
    second_call_time = time.time() - start_time

    print(f"First call: {first_call_time:.4f}s, Second call: {second_call_time:.4f}s")
    print(f"Results match: {result1 == result2}")