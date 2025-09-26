"""
Data models and ORM-like classes for the Python test project.

This module contains:
- SQLAlchemy-like model definitions
- Pydantic models for validation
- Custom field types and validators
- Model relationships and queries
"""

from __future__ import annotations

import re
import uuid
from datetime import datetime, date
from decimal import Decimal
from enum import Enum, IntEnum
from typing import Dict, List, Optional, Union, Any, ClassVar, Type, get_type_hints
from dataclasses import dataclass, field
from abc import ABC, abstractmethod

try:
    from pydantic import BaseModel, Field, validator, root_validator
    PYDANTIC_AVAILABLE = True
except ImportError:
    PYDANTIC_AVAILABLE = False
    BaseModel = object


class UserStatus(Enum):
    """Enumeration for user status."""
    ACTIVE = "active"
    INACTIVE = "inactive"
    PENDING = "pending"
    SUSPENDED = "suspended"
    DELETED = "deleted"


class OrderStatus(IntEnum):
    """Enumeration for order status."""
    PENDING = 1
    CONFIRMED = 2
    PROCESSING = 3
    SHIPPED = 4
    DELIVERED = 5
    CANCELLED = 6
    REFUNDED = 7


class Priority(Enum):
    """Task priority enumeration."""
    LOW = "low"
    MEDIUM = "medium"
    HIGH = "high"
    URGENT = "urgent"


# Base classes for ORM-like functionality

class Field:
    """Represents a database field."""

    def __init__(
        self,
        field_type: Type,
        primary_key: bool = False,
        nullable: bool = True,
        unique: bool = False,
        default: Any = None,
        max_length: Optional[int] = None
    ):
        self.field_type = field_type
        self.primary_key = primary_key
        self.nullable = nullable
        self.unique = unique
        self.default = default
        self.max_length = max_length

    def validate(self, value: Any) -> bool:
        """Validate field value."""
        if value is None and not self.nullable:
            return False

        if value is not None and not isinstance(value, self.field_type):
            return False

        if self.max_length and isinstance(value, str) and len(value) > self.max_length:
            return False

        return True


class ModelMeta(type):
    """Metaclass for model classes."""

    def __new__(mcs, name, bases, attrs):
        fields = {}
        for key, value in list(attrs.items()):
            if isinstance(value, Field):
                fields[key] = value
                attrs.pop(key)

        attrs['_fields'] = fields
        return super().__new__(mcs, name, bases, attrs)


class BaseModel(metaclass=ModelMeta):
    """Base class for all models."""

    _fields: ClassVar[Dict[str, Field]] = {}

    def __init__(self, **kwargs):
        for field_name, field_obj in self._fields.items():
            value = kwargs.get(field_name, field_obj.default)
            if not field_obj.validate(value):
                raise ValueError(f"Invalid value for field {field_name}: {value}")
            setattr(self, field_name, value)

    def to_dict(self) -> Dict[str, Any]:
        """Convert model to dictionary."""
        return {
            field_name: getattr(self, field_name, None)
            for field_name in self._fields
        }

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> BaseModel:
        """Create model from dictionary."""
        return cls(**data)

    def validate(self) -> List[str]:
        """Validate all fields and return list of errors."""
        errors = []
        for field_name, field_obj in self._fields.items():
            value = getattr(self, field_name, None)
            if not field_obj.validate(value):
                errors.append(f"Invalid value for {field_name}")
        return errors

    def __repr__(self):
        class_name = self.__class__.__name__
        attrs = ', '.join(f"{k}={v!r}" for k, v in self.to_dict().items())
        return f"{class_name}({attrs})"


# Model definitions

class User(BaseModel):
    """User model with validation and relationships."""

    id = Field(int, primary_key=True)
    username = Field(str, nullable=False, unique=True, max_length=50)
    email = Field(str, nullable=False, unique=True, max_length=255)
    first_name = Field(str, nullable=False, max_length=100)
    last_name = Field(str, nullable=False, max_length=100)
    password_hash = Field(str, nullable=False, max_length=255)
    status = Field(UserStatus, default=UserStatus.ACTIVE)
    is_admin = Field(bool, default=False)
    last_login = Field(datetime, nullable=True)
    created_at = Field(datetime, default=datetime.now)
    updated_at = Field(datetime, default=datetime.now)

    def __init__(self, **kwargs):
        super().__init__(**kwargs)
        self._orders: List[Order] = []
        self._profile: Optional[UserProfile] = None

    @property
    def full_name(self) -> str:
        """Get user's full name."""
        return f"{self.first_name} {self.last_name}"

    @property
    def is_active(self) -> bool:
        """Check if user is active."""
        return self.status == UserStatus.ACTIVE

    def set_password(self, password: str) -> None:
        """Set user password (simplified)."""
        if len(password) < 8:
            raise ValueError("Password must be at least 8 characters long")
        # In a real app, would hash the password
        self.password_hash = f"hashed_{password}"

    def check_password(self, password: str) -> bool:
        """Check password (simplified)."""
        return self.password_hash == f"hashed_{password}"

    def get_orders(self) -> List[Order]:
        """Get user's orders."""
        return self._orders.copy()

    def add_order(self, order: Order) -> None:
        """Add an order to the user."""
        if order not in self._orders:
            self._orders.append(order)
            order.user_id = self.id


class UserProfile(BaseModel):
    """Extended user profile information."""

    id = Field(int, primary_key=True)
    user_id = Field(int, nullable=False, unique=True)
    bio = Field(str, nullable=True, max_length=1000)
    avatar_url = Field(str, nullable=True, max_length=500)
    date_of_birth = Field(date, nullable=True)
    phone_number = Field(str, nullable=True, max_length=20)
    address = Field(str, nullable=True, max_length=500)
    city = Field(str, nullable=True, max_length=100)
    country = Field(str, nullable=True, max_length=100)
    timezone = Field(str, default="UTC", max_length=50)
    preferences = Field(dict, default=dict)
    created_at = Field(datetime, default=datetime.now)
    updated_at = Field(datetime, default=datetime.now)

    @property
    def age(self) -> Optional[int]:
        """Calculate age from date of birth."""
        if not self.date_of_birth:
            return None
        today = date.today()
        return today.year - self.date_of_birth.year - (
            (today.month, today.day) < (self.date_of_birth.month, self.date_of_birth.day)
        )


class Category(BaseModel):
    """Product category model."""

    id = Field(int, primary_key=True)
    name = Field(str, nullable=False, unique=True, max_length=100)
    description = Field(str, nullable=True, max_length=500)
    parent_id = Field(int, nullable=True)
    slug = Field(str, nullable=False, unique=True, max_length=100)
    is_active = Field(bool, default=True)
    sort_order = Field(int, default=0)
    created_at = Field(datetime, default=datetime.now)
    updated_at = Field(datetime, default=datetime.now)

    def __init__(self, **kwargs):
        super().__init__(**kwargs)
        self._children: List[Category] = []
        self._products: List[Product] = []

    @property
    def is_root_category(self) -> bool:
        """Check if this is a root category."""
        return self.parent_id is None

    def get_children(self) -> List[Category]:
        """Get child categories."""
        return self._children.copy()

    def add_child(self, child_category: Category) -> None:
        """Add a child category."""
        if child_category not in self._children:
            self._children.append(child_category)
            child_category.parent_id = self.id


class Product(BaseModel):
    """Product model with comprehensive fields."""

    id = Field(int, primary_key=True)
    sku = Field(str, nullable=False, unique=True, max_length=50)
    name = Field(str, nullable=False, max_length=255)
    description = Field(str, nullable=True, max_length=2000)
    short_description = Field(str, nullable=True, max_length=500)
    price = Field(Decimal, nullable=False)
    compare_price = Field(Decimal, nullable=True)
    cost_price = Field(Decimal, nullable=True)
    category_id = Field(int, nullable=False)
    brand = Field(str, nullable=True, max_length=100)
    weight = Field(float, nullable=True)
    dimensions = Field(str, nullable=True, max_length=100)
    color = Field(str, nullable=True, max_length=50)
    size = Field(str, nullable=True, max_length=50)
    material = Field(str, nullable=True, max_length=100)
    in_stock = Field(bool, default=True)
    stock_quantity = Field(int, default=0)
    low_stock_threshold = Field(int, default=10)
    is_digital = Field(bool, default=False)
    is_featured = Field(bool, default=False)
    is_active = Field(bool, default=True)
    tags = Field(list, default=list)
    images = Field(list, default=list)
    metadata = Field(dict, default=dict)
    seo_title = Field(str, nullable=True, max_length=255)
    seo_description = Field(str, nullable=True, max_length=500)
    created_at = Field(datetime, default=datetime.now)
    updated_at = Field(datetime, default=datetime.now)

    @property
    def is_available(self) -> bool:
        """Check if product is available for purchase."""
        return self.is_active and self.in_stock and self.stock_quantity > 0

    @property
    def is_on_sale(self) -> bool:
        """Check if product is on sale."""
        return self.compare_price is not None and self.price < self.compare_price

    @property
    def discount_percentage(self) -> float:
        """Calculate discount percentage if on sale."""
        if not self.is_on_sale:
            return 0.0
        return float((self.compare_price - self.price) / self.compare_price * 100)

    @property
    def is_low_stock(self) -> bool:
        """Check if product stock is low."""
        return self.stock_quantity <= self.low_stock_threshold

    def reduce_stock(self, quantity: int) -> None:
        """Reduce stock quantity."""
        if quantity <= 0:
            raise ValueError("Quantity must be positive")
        if quantity > self.stock_quantity:
            raise ValueError("Insufficient stock")
        self.stock_quantity -= quantity
        if self.stock_quantity == 0:
            self.in_stock = False

    def increase_stock(self, quantity: int) -> None:
        """Increase stock quantity."""
        if quantity <= 0:
            raise ValueError("Quantity must be positive")
        self.stock_quantity += quantity
        if self.stock_quantity > 0:
            self.in_stock = True


class Order(BaseModel):
    """Order model with comprehensive order management."""

    id = Field(int, primary_key=True)
    order_number = Field(str, nullable=False, unique=True, max_length=50)
    user_id = Field(int, nullable=False)
    status = Field(OrderStatus, default=OrderStatus.PENDING)
    subtotal = Field(Decimal, nullable=False)
    tax_amount = Field(Decimal, default=Decimal('0.00'))
    shipping_amount = Field(Decimal, default=Decimal('0.00'))
    discount_amount = Field(Decimal, default=Decimal('0.00'))
    total_amount = Field(Decimal, nullable=False)
    currency = Field(str, default="USD", max_length=3)
    payment_status = Field(str, default="pending", max_length=50)
    payment_method = Field(str, nullable=True, max_length=50)
    shipping_method = Field(str, nullable=True, max_length=100)
    notes = Field(str, nullable=True, max_length=1000)
    shipped_at = Field(datetime, nullable=True)
    delivered_at = Field(datetime, nullable=True)
    created_at = Field(datetime, default=datetime.now)
    updated_at = Field(datetime, default=datetime.now)

    def __init__(self, **kwargs):
        super().__init__(**kwargs)
        self._items: List[OrderItem] = []
        self._shipping_address: Optional[Address] = None
        self._billing_address: Optional[Address] = None

    @classmethod
    def generate_order_number(cls) -> str:
        """Generate a unique order number."""
        timestamp = datetime.now().strftime("%Y%m%d%H%M%S")
        random_suffix = str(uuid.uuid4().hex)[:6].upper()
        return f"ORD-{timestamp}-{random_suffix}"

    @property
    def is_paid(self) -> bool:
        """Check if order is paid."""
        return self.payment_status == "paid"

    @property
    def is_shipped(self) -> bool:
        """Check if order is shipped."""
        return self.status.value >= OrderStatus.SHIPPED.value

    @property
    def is_delivered(self) -> bool:
        """Check if order is delivered."""
        return self.status == OrderStatus.DELIVERED

    @property
    def is_cancellable(self) -> bool:
        """Check if order can be cancelled."""
        return self.status.value <= OrderStatus.PROCESSING.value

    def add_item(self, product_id: int, quantity: int, unit_price: Decimal) -> OrderItem:
        """Add an item to the order."""
        item = OrderItem(
            order_id=self.id,
            product_id=product_id,
            quantity=quantity,
            unit_price=unit_price
        )
        self._items.append(item)
        self._recalculate_totals()
        return item

    def remove_item(self, product_id: int) -> bool:
        """Remove an item from the order."""
        for i, item in enumerate(self._items):
            if item.product_id == product_id:
                del self._items[i]
                self._recalculate_totals()
                return True
        return False

    def _recalculate_totals(self) -> None:
        """Recalculate order totals."""
        self.subtotal = sum(item.line_total for item in self._items)
        self.total_amount = (
            self.subtotal + self.tax_amount +
            self.shipping_amount - self.discount_amount
        )

    def get_items(self) -> List[OrderItem]:
        """Get order items."""
        return self._items.copy()


class OrderItem(BaseModel):
    """Individual item within an order."""

    id = Field(int, primary_key=True)
    order_id = Field(int, nullable=False)
    product_id = Field(int, nullable=False)
    quantity = Field(int, nullable=False)
    unit_price = Field(Decimal, nullable=False)
    line_total = Field(Decimal, nullable=False)
    product_snapshot = Field(dict, default=dict)  # Store product data at time of order
    created_at = Field(datetime, default=datetime.now)

    def __init__(self, **kwargs):
        if 'line_total' not in kwargs and 'quantity' in kwargs and 'unit_price' in kwargs:
            kwargs['line_total'] = Decimal(str(kwargs['quantity'])) * kwargs['unit_price']
        super().__init__(**kwargs)

    def update_quantity(self, new_quantity: int) -> None:
        """Update item quantity and recalculate total."""
        if new_quantity <= 0:
            raise ValueError("Quantity must be positive")
        self.quantity = new_quantity
        self.line_total = Decimal(str(self.quantity)) * self.unit_price


class Address(BaseModel):
    """Address model for shipping and billing."""

    id = Field(int, primary_key=True)
    first_name = Field(str, nullable=False, max_length=100)
    last_name = Field(str, nullable=False, max_length=100)
    company = Field(str, nullable=True, max_length=200)
    address_line_1 = Field(str, nullable=False, max_length=255)
    address_line_2 = Field(str, nullable=True, max_length=255)
    city = Field(str, nullable=False, max_length=100)
    state_province = Field(str, nullable=True, max_length=100)
    postal_code = Field(str, nullable=False, max_length=20)
    country = Field(str, nullable=False, max_length=100)
    phone = Field(str, nullable=True, max_length=20)
    is_default = Field(bool, default=False)
    created_at = Field(datetime, default=datetime.now)
    updated_at = Field(datetime, default=datetime.now)

    @property
    def full_name(self) -> str:
        """Get full name."""
        return f"{self.first_name} {self.last_name}"

    def format_address(self, single_line: bool = False) -> str:
        """Format address as string."""
        parts = [
            self.full_name,
            self.company,
            self.address_line_1,
            self.address_line_2,
            f"{self.city}, {self.state_province} {self.postal_code}",
            self.country
        ]

        # Filter out empty parts
        parts = [part for part in parts if part and part.strip()]

        separator = ", " if single_line else "\n"
        return separator.join(parts)


# Pydantic models for API serialization (if Pydantic is available)
if PYDANTIC_AVAILABLE:

    class UserCreateRequest(BaseModel):
        """Pydantic model for user creation request."""
        username: str = Field(..., min_length=3, max_length=50)
        email: str = Field(..., regex=r'^[\w\.-]+@[\w\.-]+\.\w+$')
        first_name: str = Field(..., min_length=1, max_length=100)
        last_name: str = Field(..., min_length=1, max_length=100)
        password: str = Field(..., min_length=8)

        @validator('username')
        def username_alphanumeric(cls, v):
            assert v.isalnum(), 'Username must be alphanumeric'
            return v

        @validator('password')
        def validate_password_strength(cls, v):
            if not re.search(r'[A-Z]', v):
                raise ValueError('Password must contain uppercase letter')
            if not re.search(r'[a-z]', v):
                raise ValueError('Password must contain lowercase letter')
            if not re.search(r'\d', v):
                raise ValueError('Password must contain digit')
            return v

    class ProductResponse(BaseModel):
        """Pydantic model for product API response."""
        id: int
        sku: str
        name: str
        description: Optional[str]
        price: Decimal
        category_id: int
        in_stock: bool
        stock_quantity: int
        is_featured: bool
        tags: List[str]
        created_at: datetime

        class Config:
            json_encoders = {
                Decimal: lambda v: float(v),
                datetime: lambda v: v.isoformat()
            }


# Repository pattern for data access
class Repository(ABC):
    """Abstract base repository class."""

    @abstractmethod
    async def get_by_id(self, id: int) -> Optional[BaseModel]:
        """Get entity by ID."""
        pass

    @abstractmethod
    async def create(self, entity: BaseModel) -> BaseModel:
        """Create new entity."""
        pass

    @abstractmethod
    async def update(self, entity: BaseModel) -> BaseModel:
        """Update existing entity."""
        pass

    @abstractmethod
    async def delete(self, id: int) -> bool:
        """Delete entity by ID."""
        pass

    @abstractmethod
    async def list(self, limit: int = 100, offset: int = 0) -> List[BaseModel]:
        """List entities with pagination."""
        pass


class InMemoryRepository(Repository):
    """In-memory repository implementation for testing."""

    def __init__(self, model_class: Type[BaseModel]):
        self.model_class = model_class
        self._data: Dict[int, BaseModel] = {}
        self._next_id = 1

    async def get_by_id(self, id: int) -> Optional[BaseModel]:
        return self._data.get(id)

    async def create(self, entity: BaseModel) -> BaseModel:
        entity.id = self._next_id
        entity.created_at = datetime.now()
        entity.updated_at = datetime.now()
        self._data[entity.id] = entity
        self._next_id += 1
        return entity

    async def update(self, entity: BaseModel) -> BaseModel:
        if entity.id not in self._data:
            raise ValueError(f"Entity with id {entity.id} not found")
        entity.updated_at = datetime.now()
        self._data[entity.id] = entity
        return entity

    async def delete(self, id: int) -> bool:
        if id in self._data:
            del self._data[id]
            return True
        return False

    async def list(self, limit: int = 100, offset: int = 0) -> List[BaseModel]:
        items = list(self._data.values())
        return items[offset:offset + limit]

    async def count(self) -> int:
        """Count total entities."""
        return len(self._data)

    def clear(self) -> None:
        """Clear all data (useful for testing)."""
        self._data.clear()
        self._next_id = 1