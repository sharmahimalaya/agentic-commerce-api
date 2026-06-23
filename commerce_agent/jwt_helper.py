import time
from typing import Dict, Any
from jose import jwt 
from .config import settings

if not settings.mandate_private_key_pem or not settings.mandate_public_key_pem:
    raise ValueError(
        "Mandate private and public keys must be configured in environment variables or .env. "
        "Please define MANDATE_PRIVATE_KEY_PEM and MANDATE_PUBLIC_KEY_PEM."
    )

PRIVATE_KEY = settings.mandate_private_key_pem.replace("\\n", "\n")
PUBLIC_KEY = settings.mandate_public_key_pem.replace("\\n", "\n")


ALGORITHM = "RS256"
ISSUER = "commerce_api"

def sign_mandate(payload:Dict[str, Any]) -> str:
    """
    Returns a signed JWT that will travel to the Go server.
    Required fields in *payload*:
        - mandate_id (str)
        - cart_hash   (str)   # hash of the cart items + amount
        - amount_pa   (int)   # total amount in paise
        - iat, exp   (int)   # automatically added
    """
    now = int(time.time())
    full_payload = {
        "iss": ISSUER,
        "iat": now,
        "exp": now+300,
        **payload,
    }
    return jwt.encode(full_payload, PRIVATE_KEY, algorithm=ALGORITHM)

def verify_mandate(token:str) -> Dict[str, Any]:
    """Used on the Go side – we expose the public key for verification."""
    return jwt.decode(token, PUBLIC_KEY, algorithms=[ALGORITHM], issuer=ISSUER)