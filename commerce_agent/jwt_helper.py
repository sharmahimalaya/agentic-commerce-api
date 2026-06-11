import time
from typing import Dict, Any
from jose import jwt 

PRIVATE_KEY = """-----BEGIN PRIVATE KEY-----
MIIEvgIBADANBgkqhkiG9w0BAQEFAASCBKgwggSkAgEAAoIBAQDcQcm6JxkGYwFd
xm4QYao4ME78LnaoCcUJ+0pY9UDehNIbmLuwdJ/POfpIRadZlBjHImBftQx2XniG
/snZJY4h/BG1iKmNJnA+U0kwUiovn+rPooxeoU3FpLjHjlR98QI5J2brlp9Xwk0l
crExu+JQKMsw0VXbGvywkh0YSLM2ZbzmzgfDpWwj0uqXbuGjH0hQvnhKSoo5xHQC
O18bbs8q79W2vsLzOlV/zzjrxywbHuX3CIkNlhuv+R4vVuT0RCKaWMgabuwPtZbW
+MZ/ymAmMxj3bsPiw3b0aSV4PuicZ+SuOLj40bev9Llm9F6zRXmYK0oCxQ7mtGe3
9Fqz3w+XAgMBAAECggEADh3SowaseOym2BlxURrMARaU3rQk+2Ic6dCG6aqrrKyl
7Ao0RVFEM3uLdKRaNHM5yfwvDGibKDRSfzxx4rlLIcHOahcciXpkM+pnRMc6AvFk
kvLfvOo+WiN+NZ6uqvUOZ8FZvFGxXBDiRuXR64EXi9GsLDB5KIt96dyDgYdEooDG
RtshE98h4Scrn8wSIFtSWU1BJyvLP1PVBgaG2foqN7aj9vmPx4/TrgbpmouO5RpG
CPvEA0sxbzwaaND3E+2AY91iM4B82ZcYFJCCcsbZe9dl+M/P4H813fvEGsJ2sFIu
biYt6GKunhHheOdFrM0Md+1T5Ry5VijHv4+3pKk9SQKBgQD7um7uzhRpyam09WlI
Y56a9twHDfKDluj0px4ie3GmzEVAew7YDRdQZU3FsTXZpOTcSep87fQZC7f+CFyg
Y0/FznaecX3AJKgHu6v61m1AcHeAgLlfK5JB6BLg7uuyjpRhydzdL4rmsACt13Vn
1y1IdmoanBLI+9vPbadDIpRrEwKBgQDf/qLVpx05WaE1Xuq5tQCk8WdsJrC9c3nb
wtVtg8ocxt/+G0/wrnRpKwG0zP6TAXDh121XIEU5XeNejBEWx+TXlVlkdgJGwCts
Tlu/SsZ8fasR4PQL0J9RNuaWDV0eRvuMdgGxfq53ug6GOHAUAi7or+BcSu1yNokq
YYjO5R817QKBgFpLYYdffIsFv04dyYoh0b6cVghhxF/XPfCkEXck+HtwQlcCzSxK
ZdZ8wAztp/dN4pnyGZ5+bFSfk3wX28HcXb0CdiIXa5gEjhFYDDSJvd6jePorMlMk
+e2SJVNx4DHIWwlIs2TTrOtarqOs6Xw5/xBDCYRJ/6MAVLRvDNRUDxDpAoGBANeW
OFlUh68cEinRGi/1AxK9+eHA91jQXNfkFRFbx9qcmxfyZ6Vp80cJipHev6LzvxbP
BkDWIWpOcDkerI/1gs7vwuMLJbO8385VOL7LlHBbb5w8nAcHG1/KbHK9mAM9JH0T
Uxvnpro7TCFpDo5jb4yrQlDyGMlVrf0pdMhVBA4dAoGBAMaS2BsSS1hk7StJKk7t
AsqrCbxv+YIA1rRkYX0cFJ/4sGxw8PBM0/Q4y86qm4rir55Vp1kK8oT5RobSKVsz
3OeP0WvhDuOquDRw2G/Ifnd/c0Y+oj+YGPGFtKPJjhM49kwm9RdJ/wLDxa8CLXZ2
amF08PXfcJMskGhHUHTq4y/F
-----END PRIVATE KEY-----"""

PUBLIC_KEY = """-----BEGIN PUBLIC KEY-----
MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA3EHJuicZBmMBXcZuEGGq
ODBO/C52qAnFCftKWPVA3oTSG5i7sHSfzzn6SEWnWZQYxyJgX7UMdl54hv7J2SWO
IfwRtYipjSZwPlNJMFIqL5/qz6KMXqFNxaS4x45UffECOSdm65afV8JNJXKxMbvi
UCjLMNFV2xr8sJIdGEizNmW85s4Hw6VsI9Lql27hox9IUL54SkqKOcR0AjtfG27P
Ku/Vtr7C8zpVf88468csGx7l9wiJDZYbr/keL1bk9EQimljIGm7sD7WW1vjGf8pg
JjMY927D4sN29GkleD7onGfkrji4+NG3r/S5ZvRes0V5mCtKAsUO5rRnt/Ras98P
lwIDAQAB
-----END PUBLIC KEY-----"""

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