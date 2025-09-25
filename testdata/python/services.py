"""
Service layer classes for business logic and external integrations.

This module contains:
- Email service for sending notifications
- Payment processing service
- Search service with indexing
- Notification service
- Analytics service
- File upload service
- External API clients
"""

import asyncio
import json
import logging
import smtplib
import time
from abc import ABC, abstractmethod
from datetime import datetime, timedelta
from email.mime.text import MIMEText
from email.mime.multipart import MIMEMultipart
from typing import Dict, List, Optional, Any, Union, Tuple, Protocol
from dataclasses import dataclass, field
from enum import Enum
from pathlib import Path
import uuid

# Setup logging
logger = logging.getLogger(__name__)


class ServiceError(Exception):
    """Base exception for service errors."""
    pass


class EmailServiceError(ServiceError):
    """Email service specific errors."""
    pass


class PaymentServiceError(ServiceError):
    """Payment service specific errors."""
    pass


class NotificationServiceError(ServiceError):
    """Notification service specific errors."""
    pass


# Configuration classes
@dataclass
class EmailConfig:
    """Email service configuration."""
    smtp_host: str
    smtp_port: int
    username: str
    password: str
    use_tls: bool = True
    use_ssl: bool = False
    default_sender: str = ""
    timeout: int = 30

    def __post_init__(self):
        if not self.default_sender:
            self.default_sender = self.username


@dataclass
class PaymentConfig:
    """Payment service configuration."""
    api_key: str
    api_secret: str
    webhook_secret: str
    api_base_url: str = "https://api.stripe.com/v1"
    timeout: int = 30
    max_retries: int = 3


@dataclass
class SearchConfig:
    """Search service configuration."""
    index_path: str
    max_results: int = 100
    fuzzy_threshold: float = 0.8
    enable_analytics: bool = True


# Protocol definitions for dependency injection
class EmailProvider(Protocol):
    """Email provider protocol."""

    async def send_email(
        self,
        to: str,
        subject: str,
        body: str,
        html_body: Optional[str] = None
    ) -> bool:
        """Send an email."""
        ...


class PaymentProvider(Protocol):
    """Payment provider protocol."""

    async def process_payment(
        self,
        amount: int,
        currency: str,
        payment_method: str,
        metadata: Dict[str, Any] = None
    ) -> Dict[str, Any]:
        """Process a payment."""
        ...


# Enums
class NotificationType(Enum):
    """Types of notifications."""
    EMAIL = "email"
    SMS = "sms"
    PUSH = "push"
    IN_APP = "in_app"
    SLACK = "slack"
    WEBHOOK = "webhook"


class PaymentStatus(Enum):
    """Payment status enumeration."""
    PENDING = "pending"
    PROCESSING = "processing"
    SUCCEEDED = "succeeded"
    FAILED = "failed"
    CANCELLED = "cancelled"
    REFUNDED = "refunded"


class EmailTemplate(Enum):
    """Email template types."""
    WELCOME = "welcome"
    PASSWORD_RESET = "password_reset"
    ORDER_CONFIRMATION = "order_confirmation"
    SHIPPING_NOTIFICATION = "shipping_notification"
    INVOICE = "invoice"
    NEWSLETTER = "newsletter"
    ACCOUNT_VERIFICATION = "account_verification"


# Data classes
@dataclass
class EmailMessage:
    """Email message data structure."""
    to: str
    subject: str
    body: str
    html_body: Optional[str] = None
    from_email: Optional[str] = None
    reply_to: Optional[str] = None
    attachments: List[str] = field(default_factory=list)
    headers: Dict[str, str] = field(default_factory=dict)
    template: Optional[EmailTemplate] = None
    template_data: Dict[str, Any] = field(default_factory=dict)


@dataclass
class Notification:
    """Notification data structure."""
    id: str = field(default_factory=lambda: str(uuid.uuid4()))
    type: NotificationType = NotificationType.EMAIL
    recipient: str = ""
    title: str = ""
    message: str = ""
    data: Dict[str, Any] = field(default_factory=dict)
    scheduled_at: Optional[datetime] = None
    sent_at: Optional[datetime] = None
    status: str = "pending"
    retry_count: int = 0
    max_retries: int = 3


@dataclass
class PaymentIntent:
    """Payment intent data structure."""
    id: str = field(default_factory=lambda: str(uuid.uuid4()))
    amount: int  # Amount in cents
    currency: str = "usd"
    payment_method: str = ""
    customer_id: Optional[str] = None
    description: Optional[str] = None
    metadata: Dict[str, Any] = field(default_factory=dict)
    status: PaymentStatus = PaymentStatus.PENDING
    created_at: datetime = field(default_factory=datetime.now)
    updated_at: datetime = field(default_factory=datetime.now)


@dataclass
class SearchResult:
    """Search result data structure."""
    id: str
    title: str
    content: str
    score: float
    type: str
    url: Optional[str] = None
    metadata: Dict[str, Any] = field(default_factory=dict)
    highlighted_content: Optional[str] = None


@dataclass
class AnalyticsEvent:
    """Analytics event data structure."""
    event_name: str
    user_id: Optional[str] = None
    session_id: Optional[str] = None
    properties: Dict[str, Any] = field(default_factory=dict)
    timestamp: datetime = field(default_factory=datetime.now)
    ip_address: Optional[str] = None
    user_agent: Optional[str] = None


# Service implementations
class EmailService:
    """Email service for sending emails through SMTP."""

    def __init__(self, config: EmailConfig):
        self.config = config
        self.template_cache: Dict[EmailTemplate, str] = {}
        self._load_templates()

    def _load_templates(self):
        """Load email templates from files or define them inline."""
        # In a real app, these would be loaded from template files
        self.template_cache[EmailTemplate.WELCOME] = """
        <html>
        <body>
            <h1>Welcome {name}!</h1>
            <p>Thank you for joining us. Your account has been created successfully.</p>
            <p>Your username is: {username}</p>
        </body>
        </html>
        """

        self.template_cache[EmailTemplate.PASSWORD_RESET] = """
        <html>
        <body>
            <h1>Password Reset Request</h1>
            <p>Hi {name},</p>
            <p>You requested a password reset. Click the link below to reset your password:</p>
            <a href="{reset_url}">Reset Password</a>
            <p>This link expires in 24 hours.</p>
        </body>
        </html>
        """

        self.template_cache[EmailTemplate.ORDER_CONFIRMATION] = """
        <html>
        <body>
            <h1>Order Confirmation</h1>
            <p>Hi {customer_name},</p>
            <p>Thank you for your order! Here are the details:</p>
            <ul>
                <li>Order Number: {order_number}</li>
                <li>Total Amount: ${total_amount}</li>
                <li>Estimated Delivery: {delivery_date}</li>
            </ul>
        </body>
        </html>
        """

    def _render_template(self, template: EmailTemplate, data: Dict[str, Any]) -> str:
        """Render email template with data."""
        if template not in self.template_cache:
            raise EmailServiceError(f"Template {template.value} not found")

        template_content = self.template_cache[template]
        try:
            return template_content.format(**data)
        except KeyError as e:
            raise EmailServiceError(f"Missing template variable: {e}")

    async def send_email(self, message: EmailMessage) -> bool:
        """Send an email message."""
        try:
            # Render template if specified
            body = message.body
            html_body = message.html_body

            if message.template:
                html_body = self._render_template(message.template, message.template_data)
                if not body:
                    # Create plain text version (simplified)
                    import re
                    body = re.sub(r'<[^>]+>', '', html_body)

            # Create message
            msg = MIMEMultipart('alternative')
            msg['Subject'] = message.subject
            msg['From'] = message.from_email or self.config.default_sender
            msg['To'] = message.to

            if message.reply_to:
                msg['Reply-To'] = message.reply_to

            # Add custom headers
            for key, value in message.headers.items():
                msg[key] = value

            # Add text and HTML parts
            if body:
                text_part = MIMEText(body, 'plain')
                msg.attach(text_part)

            if html_body:
                html_part = MIMEText(html_body, 'html')
                msg.attach(html_part)

            # Send email
            await self._send_smtp(msg)
            logger.info(f"Email sent successfully to {message.to}")
            return True

        except Exception as e:
            logger.error(f"Failed to send email to {message.to}: {e}")
            raise EmailServiceError(f"Failed to send email: {e}")

    async def _send_smtp(self, message: MIMEMultipart):
        """Send email via SMTP."""
        # Simulate async SMTP sending
        await asyncio.sleep(0.1)

        # In a real implementation, would use aiosmtplib or similar
        logger.debug(f"Sending email via SMTP to {self.config.smtp_host}:{self.config.smtp_port}")

    async def send_bulk_emails(self, messages: List[EmailMessage]) -> Dict[str, bool]:
        """Send multiple emails in bulk."""
        results = {}

        # Send emails in batches to avoid overwhelming the server
        batch_size = 10
        for i in range(0, len(messages), batch_size):
            batch = messages[i:i + batch_size]
            tasks = [self.send_email(msg) for msg in batch]
            batch_results = await asyncio.gather(*tasks, return_exceptions=True)

            for msg, result in zip(batch, batch_results):
                results[msg.to] = isinstance(result, bool) and result

            # Small delay between batches
            if i + batch_size < len(messages):
                await asyncio.sleep(0.5)

        return results


class PaymentService:
    """Payment processing service."""

    def __init__(self, config: PaymentConfig):
        self.config = config
        self.payment_history: List[PaymentIntent] = []

    async def create_payment_intent(
        self,
        amount: int,
        currency: str = "usd",
        customer_id: Optional[str] = None,
        description: Optional[str] = None,
        metadata: Dict[str, Any] = None
    ) -> PaymentIntent:
        """Create a payment intent."""
        intent = PaymentIntent(
            amount=amount,
            currency=currency,
            customer_id=customer_id,
            description=description,
            metadata=metadata or {}
        )

        logger.info(f"Created payment intent {intent.id} for {amount} {currency}")
        return intent

    async def process_payment(self, intent: PaymentIntent, payment_method: str) -> PaymentIntent:
        """Process a payment."""
        intent.payment_method = payment_method
        intent.status = PaymentStatus.PROCESSING
        intent.updated_at = datetime.now()

        try:
            # Simulate payment processing
            await asyncio.sleep(1.0)  # Simulate API call delay

            # Simulate different outcomes based on amount
            if intent.amount < 100:  # Less than $1.00
                intent.status = PaymentStatus.FAILED
                raise PaymentServiceError("Amount too small")
            elif intent.amount > 1000000:  # More than $10,000
                intent.status = PaymentStatus.FAILED
                raise PaymentServiceError("Amount exceeds limit")
            else:
                intent.status = PaymentStatus.SUCCEEDED

            intent.updated_at = datetime.now()
            self.payment_history.append(intent)

            logger.info(f"Payment {intent.id} processed successfully")
            return intent

        except Exception as e:
            intent.status = PaymentStatus.FAILED
            intent.updated_at = datetime.now()
            self.payment_history.append(intent)

            logger.error(f"Payment {intent.id} failed: {e}")
            raise PaymentServiceError(f"Payment processing failed: {e}")

    async def refund_payment(self, payment_id: str, amount: Optional[int] = None) -> Dict[str, Any]:
        """Refund a payment."""
        # Find the original payment
        payment = next((p for p in self.payment_history if p.id == payment_id), None)
        if not payment:
            raise PaymentServiceError(f"Payment {payment_id} not found")

        if payment.status != PaymentStatus.SUCCEEDED:
            raise PaymentServiceError("Can only refund succeeded payments")

        refund_amount = amount or payment.amount
        if refund_amount > payment.amount:
            raise PaymentServiceError("Refund amount cannot exceed original payment")

        # Process refund
        await asyncio.sleep(0.5)  # Simulate API call

        refund_id = str(uuid.uuid4())
        refund = {
            'id': refund_id,
            'payment_id': payment_id,
            'amount': refund_amount,
            'status': 'succeeded',
            'created_at': datetime.now().isoformat()
        }

        # Update original payment status if fully refunded
        if refund_amount == payment.amount:
            payment.status = PaymentStatus.REFUNDED

        logger.info(f"Refund {refund_id} processed for payment {payment_id}")
        return refund

    def get_payment_history(self, customer_id: Optional[str] = None) -> List[PaymentIntent]:
        """Get payment history."""
        if customer_id:
            return [p for p in self.payment_history if p.customer_id == customer_id]
        return self.payment_history.copy()


class SearchService:
    """Search service with indexing and full-text search."""

    def __init__(self, config: SearchConfig):
        self.config = config
        self.search_index: Dict[str, Dict[str, Any]] = {}
        self.search_analytics: List[Dict[str, Any]] = []

    def index_document(
        self,
        doc_id: str,
        title: str,
        content: str,
        doc_type: str = "document",
        metadata: Dict[str, Any] = None,
        url: Optional[str] = None
    ):
        """Index a document for searching."""
        self.search_index[doc_id] = {
            'id': doc_id,
            'title': title,
            'content': content,
            'type': doc_type,
            'metadata': metadata or {},
            'url': url,
            'indexed_at': datetime.now(),
            'word_count': len(content.split())
        }

        logger.debug(f"Indexed document {doc_id}: {title}")

    def remove_document(self, doc_id: str) -> bool:
        """Remove a document from the index."""
        if doc_id in self.search_index:
            del self.search_index[doc_id]
            logger.debug(f"Removed document {doc_id} from index")
            return True
        return False

    async def search(
        self,
        query: str,
        limit: int = 10,
        offset: int = 0,
        doc_type: Optional[str] = None,
        user_id: Optional[str] = None
    ) -> Tuple[List[SearchResult], int]:
        """Search documents in the index."""
        # Record search analytics
        if self.config.enable_analytics:
            self.search_analytics.append({
                'query': query,
                'user_id': user_id,
                'timestamp': datetime.now(),
                'doc_type_filter': doc_type
            })

        # Simple keyword-based search (in a real app, would use Elasticsearch, Solr, etc.)
        query_words = query.lower().split()
        results = []

        for doc_id, doc_data in self.search_index.items():
            # Apply document type filter
            if doc_type and doc_data['type'] != doc_type:
                continue

            # Calculate relevance score
            score = self._calculate_relevance_score(query_words, doc_data)

            if score > 0:
                result = SearchResult(
                    id=doc_data['id'],
                    title=doc_data['title'],
                    content=doc_data['content'],
                    score=score,
                    type=doc_data['type'],
                    url=doc_data['url'],
                    metadata=doc_data['metadata'],
                    highlighted_content=self._highlight_content(query_words, doc_data['content'])
                )
                results.append(result)

        # Sort by relevance score
        results.sort(key=lambda x: x.score, reverse=True)

        # Apply pagination
        total_count = len(results)
        paginated_results = results[offset:offset + limit]

        logger.info(f"Search query '{query}' returned {len(paginated_results)} results")
        return paginated_results, total_count

    def _calculate_relevance_score(self, query_words: List[str], doc_data: Dict[str, Any]) -> float:
        """Calculate relevance score for a document."""
        title_lower = doc_data['title'].lower()
        content_lower = doc_data['content'].lower()

        score = 0.0

        for word in query_words:
            # Title matches have higher weight
            title_count = title_lower.count(word)
            content_count = content_lower.count(word)

            score += title_count * 3.0  # Title matches weighted 3x
            score += content_count * 1.0  # Content matches weighted 1x

        # Normalize by document length
        if doc_data['word_count'] > 0:
            score /= (doc_data['word_count'] / 100)  # Normalize per 100 words

        return score

    def _highlight_content(self, query_words: List[str], content: str, max_length: int = 200) -> str:
        """Highlight search terms in content snippet."""
        # Find the first occurrence of any query word
        content_lower = content.lower()
        first_pos = len(content)

        for word in query_words:
            pos = content_lower.find(word)
            if pos != -1 and pos < first_pos:
                first_pos = pos

        # Extract snippet around the first match
        start = max(0, first_pos - 50)
        end = min(len(content), start + max_length)
        snippet = content[start:end]

        # Add ellipsis if truncated
        if start > 0:
            snippet = "..." + snippet
        if end < len(content):
            snippet = snippet + "..."

        # Highlight query words (simple implementation)
        for word in query_words:
            snippet = snippet.replace(word, f"**{word}**")
            snippet = snippet.replace(word.capitalize(), f"**{word.capitalize()}**")

        return snippet

    def get_search_analytics(self, days: int = 30) -> Dict[str, Any]:
        """Get search analytics for the past N days."""
        cutoff_date = datetime.now() - timedelta(days=days)
        recent_searches = [
            s for s in self.search_analytics
            if s['timestamp'] >= cutoff_date
        ]

        # Calculate popular queries
        query_counts = {}
        for search in recent_searches:
            query = search['query']
            query_counts[query] = query_counts.get(query, 0) + 1

        popular_queries = sorted(
            query_counts.items(),
            key=lambda x: x[1],
            reverse=True
        )[:10]

        return {
            'total_searches': len(recent_searches),
            'unique_queries': len(query_counts),
            'popular_queries': popular_queries,
            'period_days': days
        }


class NotificationService:
    """Service for managing and sending notifications."""

    def __init__(self, email_service: Optional[EmailService] = None):
        self.email_service = email_service
        self.notification_queue: List[Notification] = []
        self.sent_notifications: List[Notification] = []

    async def send_notification(self, notification: Notification) -> bool:
        """Send a notification immediately."""
        try:
            if notification.type == NotificationType.EMAIL:
                if not self.email_service:
                    raise NotificationServiceError("Email service not configured")

                email_message = EmailMessage(
                    to=notification.recipient,
                    subject=notification.title,
                    body=notification.message,
                    template_data=notification.data
                )

                success = await self.email_service.send_email(email_message)

                if success:
                    notification.status = "sent"
                    notification.sent_at = datetime.now()
                else:
                    notification.status = "failed"

            elif notification.type == NotificationType.SMS:
                # Simulate SMS sending
                await asyncio.sleep(0.1)
                notification.status = "sent"
                notification.sent_at = datetime.now()
                logger.info(f"SMS sent to {notification.recipient}: {notification.message}")

            elif notification.type == NotificationType.PUSH:
                # Simulate push notification
                await asyncio.sleep(0.1)
                notification.status = "sent"
                notification.sent_at = datetime.now()
                logger.info(f"Push notification sent to {notification.recipient}")

            else:
                raise NotificationServiceError(f"Unsupported notification type: {notification.type}")

            if notification.status == "sent":
                self.sent_notifications.append(notification)
                return True
            else:
                return False

        except Exception as e:
            notification.status = "failed"
            notification.retry_count += 1
            logger.error(f"Failed to send notification {notification.id}: {e}")
            return False

    def schedule_notification(self, notification: Notification):
        """Schedule a notification for later sending."""
        if not notification.scheduled_at:
            notification.scheduled_at = datetime.now() + timedelta(minutes=5)

        notification.status = "scheduled"
        self.notification_queue.append(notification)
        logger.info(f"Scheduled notification {notification.id} for {notification.scheduled_at}")

    async def process_scheduled_notifications(self):
        """Process notifications that are ready to be sent."""
        now = datetime.now()
        ready_notifications = [
            n for n in self.notification_queue
            if n.scheduled_at and n.scheduled_at <= now and n.status == "scheduled"
        ]

        for notification in ready_notifications:
            success = await self.send_notification(notification)

            if success or notification.retry_count >= notification.max_retries:
                # Remove from queue if sent successfully or max retries reached
                self.notification_queue.remove(notification)
            else:
                # Reschedule for retry
                notification.scheduled_at = now + timedelta(minutes=5 * (notification.retry_count + 1))

        logger.info(f"Processed {len(ready_notifications)} scheduled notifications")

    def get_notification_stats(self) -> Dict[str, Any]:
        """Get notification statistics."""
        total_sent = len(self.sent_notifications)
        queued = len([n for n in self.notification_queue if n.status == "scheduled"])
        failed = len([n for n in self.notification_queue if n.status == "failed"])

        # Group by type
        type_stats = {}
        for notification in self.sent_notifications:
            notification_type = notification.type.value
            type_stats[notification_type] = type_stats.get(notification_type, 0) + 1

        return {
            'total_sent': total_sent,
            'queued': queued,
            'failed': failed,
            'by_type': type_stats
        }


class AnalyticsService:
    """Service for collecting and analyzing user analytics."""

    def __init__(self):
        self.events: List[AnalyticsEvent] = []

    def track_event(
        self,
        event_name: str,
        user_id: Optional[str] = None,
        session_id: Optional[str] = None,
        properties: Dict[str, Any] = None,
        ip_address: Optional[str] = None,
        user_agent: Optional[str] = None
    ):
        """Track an analytics event."""
        event = AnalyticsEvent(
            event_name=event_name,
            user_id=user_id,
            session_id=session_id,
            properties=properties or {},
            ip_address=ip_address,
            user_agent=user_agent
        )

        self.events.append(event)
        logger.debug(f"Tracked event: {event_name} for user {user_id}")

    def get_event_counts(self, days: int = 30) -> Dict[str, int]:
        """Get event counts for the past N days."""
        cutoff_date = datetime.now() - timedelta(days=days)
        recent_events = [e for e in self.events if e.timestamp >= cutoff_date]

        event_counts = {}
        for event in recent_events:
            event_counts[event.event_name] = event_counts.get(event.event_name, 0) + 1

        return event_counts

    def get_user_activity(self, user_id: str, days: int = 30) -> List[AnalyticsEvent]:
        """Get activity for a specific user."""
        cutoff_date = datetime.now() - timedelta(days=days)
        return [
            e for e in self.events
            if e.user_id == user_id and e.timestamp >= cutoff_date
        ]

    def get_popular_pages(self, days: int = 30) -> List[Tuple[str, int]]:
        """Get most popular pages based on page_view events."""
        cutoff_date = datetime.now() - timedelta(days=days)
        page_views = [
            e for e in self.events
            if e.event_name == "page_view" and e.timestamp >= cutoff_date
        ]

        page_counts = {}
        for event in page_views:
            page = event.properties.get('page', 'unknown')
            page_counts[page] = page_counts.get(page, 0) + 1

        return sorted(page_counts.items(), key=lambda x: x[1], reverse=True)


# Example usage and service factory
class ServiceContainer:
    """Simple dependency injection container for services."""

    def __init__(self):
        self._services = {}

    def register(self, service_type: type, instance: Any):
        """Register a service instance."""
        self._services[service_type] = instance

    def get(self, service_type: type) -> Any:
        """Get a service instance."""
        if service_type not in self._services:
            raise ValueError(f"Service {service_type.__name__} not registered")
        return self._services[service_type]


async def example_usage():
    """Example of how to use the services."""
    # Setup configurations
    email_config = EmailConfig(
        smtp_host="smtp.gmail.com",
        smtp_port=587,
        username="test@example.com",
        password="password",
        default_sender="noreply@example.com"
    )

    payment_config = PaymentConfig(
        api_key="sk_test_123",
        api_secret="secret",
        webhook_secret="whsec_123"
    )

    search_config = SearchConfig(
        index_path="/tmp/search_index",
        max_results=50
    )

    # Initialize services
    email_service = EmailService(email_config)
    payment_service = PaymentService(payment_config)
    search_service = SearchService(search_config)
    notification_service = NotificationService(email_service)
    analytics_service = AnalyticsService()

    # Setup service container
    container = ServiceContainer()
    container.register(EmailService, email_service)
    container.register(PaymentService, payment_service)
    container.register(SearchService, search_service)
    container.register(NotificationService, notification_service)
    container.register(AnalyticsService, analytics_service)

    # Example usage
    print("Testing services...")

    # Send welcome email
    welcome_email = EmailMessage(
        to="user@example.com",
        subject="Welcome!",
        body="",
        template=EmailTemplate.WELCOME,
        template_data={"name": "John Doe", "username": "johndoe"}
    )
    await email_service.send_email(welcome_email)

    # Process payment
    payment_intent = await payment_service.create_payment_intent(
        amount=2500,  # $25.00
        description="Test payment"
    )
    await payment_service.process_payment(payment_intent, "card_123")

    # Index and search documents
    search_service.index_document(
        doc_id="doc1",
        title="Python Best Practices",
        content="This document covers Python coding best practices and conventions.",
        doc_type="article"
    )

    results, count = await search_service.search("python coding")
    print(f"Search results: {len(results)} of {count}")

    # Send notification
    notification = Notification(
        type=NotificationType.EMAIL,
        recipient="user@example.com",
        title="Order Shipped",
        message="Your order has been shipped and is on its way!"
    )
    await notification_service.send_notification(notification)

    # Track analytics event
    analytics_service.track_event(
        "user_login",
        user_id="user123",
        properties={"method": "email", "success": True}
    )

    print("Service testing completed!")


if __name__ == "__main__":
    asyncio.run(example_usage())