import httpx
# from IPython.core import payload
from asyncio import timeout
import uuid
import logging
from typing import Any, Dict, List, Optional
from .jwt_helper import sign_mandate
from .config import settings
from .models import (
    TokenCreateRequest, TokenResponse, Product, Cart, 
    CartItemInput, PaymentIntent, PaymentIntentCreateRequest
)

logger = logging.getLogger("CommerceClient")

# Custom exception to store API response details like status code and response body for better debugging
class CommerceAPIException(Exception):
    def __init__(self, message: str, status_code: Optional[int] = None, response_body: Optional[str] = None):
        super().__init__(message)
        self.status_code = status_code
        self.response_body = response_body

class CommerceAPIClient:
    """
    A robust client wrapping HTTPX to communicate with the Agentic Commerce Go API.
    Handles token creation, auto-authentication headers, and idempotency key injection.
    """
    def __init__(self):
        self.base_url = settings.commerce_api_url.rstrip("/")
        self.client = httpx.Client(timeout=10.0)
        self.bearer_token: Optional[str] = None
        self.admin_key = settings.admin_api_key
    
    def bootstrap_token(self) -> None:
        """
        Requests a new API access token using the Admin API key.
        This token is required for all subsequence catalog, cart, and checkout operations.
        """

        payload = TokenCreateRequest(
            scopes=["products:read", "carts:write", "carts:read", "payments:write"],
            spend_limit_paise=settings.default_spend_limit_paise,
            duration_hours=settings.default_token_expiry_hours
        )
        headers = {
            "X-Admin-Key": self.admin_key if isinstance(self.admin_key, str) else self.admin_key.get_secret_value(),
            "X-Request-ID": str(uuid.uuid4())
        }

        try:
            response = self.client.post(
                f"{self.base_url}/tokens",
                json=payload.model_dump(),
                headers=headers
            )
            if response.status_code != 201:
                raise CommerceAPIException(
                    f"Token generation failed: HTTP {response.status_code}",
                    status_code=response.status_code,
                    response_body = response.text
                )
            token_data = TokenResponse.model_validate(response.json())
            self.bearer_token = token_data.secret
        except Exception as e:
            if not isinstance(e, CommerceAPIException):
                raise CommerceAPIException(f"Network error during bootstrapping: {str(e)}")
            raise e

    def _get_headers(self, require_idempotency: bool=False) -> Dict[str,str]:
        """Assembles common headers including auth tracing and optional idempotency keys."""
        if not self.bearer_token:
            self.bootstrap_token()
        
        headers = {
            "Authorization": f"Bearer {self.bearer_token}",
            "X-Request-ID": str(uuid.uuid4()),
            "Content-Type":"application/json"
        }
        # If the endpoint needs idempotency (e.g. POST requests), we attach a unique UUID key
        if require_idempotency:
            headers["Idempotency-Key"] = str(uuid.uuid4())
        return headers

    def get_products(self) -> List[Product]:
        """Fetches the catalog of products from the API."""

        headers = self._get_headers()
        try:
            response = self.client.get(f"{self.base_url}/products", headers=headers)
            if response.status_code != 200:
                raise CommerceAPIException("Failed to fetch products", response.status_code, response.text)
            return[Product.model_validate(p) for p in response.json()]
        except httpx.HTTPError as e:
            raise CommerceAPIException(f"HTTP Connection failure: {str(e)}")
        
    def create_cart(self) -> Cart:
        """Initializes a new shopping cart."""
        headers = self._get_headers(require_idempotency=True)
        try:
            response = self.client.post(f"{self.base_url}/carts", headers=headers)
            if response.status_code != 201:
                raise CommerceAPIException("Failed to create cart", response.status_code, response.text)
            return Cart.model_validate(response.json())
        except httpx.HTTPError as e:
            raise CommerceAPIException(f"HTTP Connection failure: {str(e)}")
        
    def get_cart(self, cart_id: str) -> Cart:
        """Retrieves details of a specific cart."""
        headers = self._get_headers()
        try:
            response = self.client.get(f"{self.base_url}/carts/{cart_id}", headers=headers)
            if response.status_code != 200:
                raise CommerceAPIException("Failed to fetch cart", response.status_code, response.text)
            return Cart.model_validate(response.json())
        except httpx.HTTPError as e:
            raise CommerceAPIException(f"HTTP Connection Failure: {str(e)}")

    def add_cart_item(self, cart_id: str, product_id: str, quantity: int) -> Cart:
        """Adds a product item to the cart."""
        headers = self._get_headers()
        payload = CartItemInput(product_id=product_id, quantity=quantity)
        try:
            response = self.client.post(
                f"{self.base_url}/carts/{cart_id}/items",
                json=payload.model_dump(),
                headers=headers
            )
            if response.status_code != 200:
                raise CommerceAPIException("Failed to add cart item", response.status_code, response.text)
            return Cart.model_validate(response.json())
        except httpx.HTTPError as e:
            raise CommerceAPIException(f"HTTP Connection failure: {str(e)}")

    def create_payment_intent(self, cart_id:str, currency: str="INR") -> PaymentIntent:
        """
        Generates a payment intent representing the cart's value.
        This builds and cryptographically signs a JWT purchase mandate using RS256,
        ensuring the API can verify the exact cart contents, total, and mandate ID.
        """
        cart = self.get_cart(cart_id)
        import hashlib
        # Recreate the exact string representaton of the cart: cartID|items|totalPaise
        hash_input = (
            f"{cart.id}|"
            f"{','.join(f'{it.product_id}:{it.quantity}' for it in cart.items)}|"
            f"{cart.total_paise}"
        ).encode()
        cart_hash = hashlib.sha256(hash_input).hexdigest()

        # Build the mandate metadata structure
        mandate_payload = {
            "mandate_id": f"mandate-{uuid.uuid4()}",
            "cart_hash": cart_hash,
            "amount_pa": cart.total_paise,
        }
        # Cryptographically sign the mandate with the private key (RS256)
        signed_jwt = sign_mandate(mandate_payload)

        payload = PaymentIntentCreateRequest(
            cart_id=cart_id,
            currency=currency,
            mandate_jwt=signed_jwt,
        )
        headers = self._get_headers(require_idempotency=True)

        try:
            response = self.client.post(
                f"{self.base_url}/payment-intents",
                json=payload.model_dump(),
                headers=headers
            )
            if response.status_code != 201:
                raise CommerceAPIException("Failed to create payment intent", response.status_code, response.text)
            intent = PaymentIntent.model_validate(response.json())
            return intent
        except httpx.HTTPError as e:
            raise CommerceAPIException(f"HTTP Connection failure: {str(e)}")
    
    def confirm_payment_intent(self, intent_id: str) -> PaymentIntent:
        """Executes payment confirmation via the gateway."""
        headers = self._get_headers()
        try:
            response = self.client.post(
                f"{self.base_url}/payment-intents/{intent_id}/confirm",
                headers = headers
            )
            if response.status_code != 200:
                raise CommerceAPIException("Failed to confirm payment intent", response.status_code, response.text)
            intent = PaymentIntent.model_validate(response.json())
            return intent
        except httpx.HTTPError as e:
            raise CommerceAPIException(f"HTTP Connection failure: {str(e)}")

