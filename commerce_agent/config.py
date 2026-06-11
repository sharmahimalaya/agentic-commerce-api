from typing import Optional
from pydantic import SecretStr
from pydantic_settings import BaseSettings, SettingsConfigDict

class AgentConfig(BaseSettings) :
    """
    Manages global configuration parameters securely.
    Uses SecretStr to prevent leakages in stack traces, console prints, or logs.
    """

    model_config = SettingsConfigDict(
        env_file=".env",
        env_file_encoding="utf-8",
        extra="ignore"
    )

    commerce_api_url: str = "http://localhost:8080/v1"
    admin_api_key: SecretStr = SecretStr("dev_admin_secret_key")
    gemini_api_key: SecretStr
    gemini_model: str = "gemini-3.1-flash-lite"  
    default_spend_limit_paise: int = 100000
    default_token_expiry_hours: int = 1        

settings = AgentConfig()