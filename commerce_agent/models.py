from typing import List, Optional
from pydantic import BaseModel, Field

class TokenCreateRequest(BaseModel):
    scopes: List[str]
    spend_limit_paise: int
    duration_hours: int

class TokenResponse(BaseModel):
    id: str
    secret: str
    scopes: List[str]
    spend_limit_paise: int
    expires_at: str

class Product(BaseModel):
    id: str
    name: str
    description: str
    price_paise: int
    stock: int

class CartItemInput(BaseModel):
    product_id: str
    quantity: int

class CartItem(BaseModel):
    product_id: str
    quantity: int
    price_paise: int

class Cart(BaseModel):
    id: str
    items: List[CartItem]
    total_paise: int
    created_at: str
    updated_at: str

class PaymentIntentCreateRequest(BaseModel):
    cart_id: str
    currency: str
    mandate_jwt: str

class PaymentIntent(BaseModel):
    id: str
    cart_id: str
    amount_paise: int
    currency: str
    status: str
    created_at: str
    updated_at: str
