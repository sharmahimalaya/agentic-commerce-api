import time
import unittest
from jose import jwt, JWTError
from commerce_agent import jwt_helper

class TestJWTHelper(unittest.TestCase):
    def test_sign_and_verify_mandate_happy_path(self):
        payload = {
            "mandate_id": "mandate_123",
            "cart_hash": "abc123hash",
            "amount_pa": 5000,
        }
        token = jwt_helper.sign_mandate(payload)
        self.assertIsNotNone(token)
        self.assertIsInstance(token, str)

        decoded = jwt_helper.verify_mandate(token)
        self.assertEqual(decoded["mandate_id"], "mandate_123")
        self.assertEqual(decoded["cart_hash"], "abc123hash")
        self.assertEqual(decoded["amount_pa"], 5000)
        self.assertEqual(decoded["iss"], jwt_helper.ISSUER)
        self.assertIn("iat", decoded)
        self.assertIn("exp", decoded)

    def test_verify_mandate_expired(self):
        now = int(time.time())
        expired_payload = {
            "iss": jwt_helper.ISSUER,
            "iat": now - 600,
            "exp": now - 300,
            "mandate_id": "mandate_expired",
            "cart_hash": "hash",
            "amount_pa": 1000,
        }
        expired_token = jwt.encode(expired_payload, jwt_helper.PRIVATE_KEY, algorithm=jwt_helper.ALGORITHM)
        
        with self.assertRaises(JWTError):
            jwt_helper.verify_mandate(expired_token)

    def test_verify_mandate_tampered(self):
        payload = {
            "mandate_id": "mandate_123",
            "cart_hash": "abc123hash",
            "amount_pa": 5000,
        }
        token = jwt_helper.sign_mandate(payload)
        
        tampered_token = token[:-5] + "AAAAA"
        
        with self.assertRaises(JWTError):
            jwt_helper.verify_mandate(tampered_token)

    def test_verify_mandate_wrong_issuer(self):
        now = int(time.time())
        bad_issuer_payload = {
            "iss": "wrong_issuer",
            "iat": now,
            "exp": now + 300,
            "mandate_id": "mandate_123",
            "cart_hash": "hash",
            "amount_pa": 1000,
        }
        bad_token = jwt.encode(bad_issuer_payload, jwt_helper.PRIVATE_KEY, algorithm=jwt_helper.ALGORITHM)
        
        with self.assertRaises(JWTError):
            jwt_helper.verify_mandate(bad_token)

if __name__ == '__main__':
    unittest.main()
