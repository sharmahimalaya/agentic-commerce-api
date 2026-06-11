import logging
from typing import List
from .client import CommerceAPIException, CommerceAPIClient

logger = logging.getLogger("CommerceTools")
api_client = CommerceAPIClient()

def list_products() -> str:
    """Retrieves the available products catalog from the database, listing their IDs, prices (in paise), and stock.
    Returns a formatted string listing all products."""
    try:
        products = api_client.get_products()
        if not products:
            return "No products are currently available in the catalog."
        result = []
        for p in products:
            price_in_rs = p.price_paise / 100
            result.append(
                f"ID: {p.id} | Name: {p.name} | Description: {p.description} | "
                f"Price: Rs. {price_in_rs:.2f} ({p.price_paise} paise) | Stock: {p.stock}"
            )
        return "\n".join(result)
    except CommerceAPIException as e:
        return f"Error retrieving products: {str(e)} (HTTP {e.status_code})"

def checkout_and_pay_cart(cart_id: str, currency: str = "INR") -> str:
    """
    Executes a complete checkout flow for a given cart. This tool:
    1. Creates a payment intent for the cart.
    2. Confirms the payment intent to capture the funds.
    Arguments:
        cart_id: The unique ID of the cart to checkout.
        currency: ISO Currency code, default is 'INR'.
    """
    try:
        intent = api_client.create_payment_intent(cart_id, currency)
        intent_id = intent.id
        confirmed = api_client.confirm_payment_intent(intent_id)
        return(
            f"Success! Cart checkout completed\n"
            f"Payment Intent ID: {confirmed.id}\n"
            f"Amount Paid: {confirmed.amount_paise / 100:.2f} {confirmed.currency}"
            f"Status: {confirmed.status}"
        )
    except CommerceAPIException as e:
        return f"Checkout failed: {str(e)} - Body: {e.response_body or 'No detail'}"

def create_cart_and_add_items(product_ids: list[str], quantities: list[int]) -> str:
    """
    Creates a new shopping cart and adds multiple items to it.
    The product_ids and quantities lists must be the same length, where each index pairs a product with its quantity.
    Arguments:
        product_ids: List of product ID strings to add. Example: ["prod_1", "prod_3"]
        quantities: List of quantities for each product. Example: [2, 1]
    """
    try:
        cart = api_client.create_cart()
        cart_id = cart.id

        added_details = []
        for pid, qty in zip(product_ids, quantities):
            cart = api_client.add_cart_item(cart_id, pid, qty)
            added_details.append(f"{qty}x{pid}")
        return (
            f"Cart successfully created with ID: {cart_id}\n"
            f"Added items: {', '.join(added_details)}\n"
            f"Cart Total: Rs. {cart.total_paise / 100:.2f} ({cart.total_paise} paise)"
        )
    except CommerceAPIException as e:
        return f"Cart setup failed: {str(e)} - Detail: {e.response_body or 'None'}"