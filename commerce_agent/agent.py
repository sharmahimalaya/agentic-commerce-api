import sys
import logging
import os
from google import genai
from google.genai import types
from .config import settings
from .tools import list_products, create_cart_and_add_items, checkout_and_pay_cart
from . import memory

try:
    import pyttsx3
except Exception:
    pyttsx3 = None

logging.basicConfig(level=logging.INFO, format="%(asctime)s [%(levelname)s] %(name)s: %(message)s")
logger = logging.getLogger("CommerceAgent")


class CommerceAgent:
    """
    An autonomous agent powered by Gemini that manages the purchase funnel
    and orchestrates execution steps via native Function Calling.
    Adds a small local memory and optional talking (TTS) support.
    """
    def __init__(self):
        if not settings.gemini_api_key and not os.environ.get("GEMINI_API_KEY"):
            logger.warning("No GEMINI_API_KEY defined. Please set it in system environment or .env")
        self.client = genai.Client(api_key=settings.gemini_api_key.get_secret_value() if settings.gemini_api_key else None)
        self.model = settings.gemini_model
        self.chat = None

        self.tools = [list_products, create_cart_and_add_items, checkout_and_pay_cart]

        self.system_instruction_base = (
            "You are an expert, friendly e-commerce AI assistant that manages user carts and checkouts.\n"
            "You have access to tools that browse the catalog, build carts, and charge payments.\n"
            "Rule: Always use the appropriate tool rather than guessing information.\n"
            "If a user asks to buy something:\n"
            "1. List products or locate the item to retrieve the correct ID.\n"
            "2. Create the cart and add those items.\n"
            "3. Complete the checkout/payment step automatically.\n"
            "Translate all currency amounts to Rs. (from Paise, where 100 paise = 1 Rs.) when replying."
        )

        # Initialize TTS engine if available
        self.tts_engine = None
        if pyttsx3 is not None:
            try:
                self.tts_engine = pyttsx3.init()
            except Exception:
                self.tts_engine = None

    def speak(self, text: str) -> None:
        if not self.tts_engine:
            return
        try:
            self.tts_engine.say(text)
            self.tts_engine.runAndWait()
        except Exception:
            # best-effort only
            pass

    def execute_task(self, prompt: str) -> str:
        """
        Runs a ReAct execution loop, automatically calling tools and
        converging on the final user solution.
        Stores brief memory and can speak responses aloud.
        """
        logger.info(f"Received user prompt: {prompt}")
        try:
            if self.chat is None:
                recent = memory.recent(5)
                memory_section = "\n".join(recent) if recent else ""
                system_instruction = self.system_instruction_base + ("\n\nRecent memory: \n" + memory_section if memory_section else "")
                config = types.GenerateContentConfig(
                    system_instruction=system_instruction,
                    tools=self.tools,
                    temperature = 0.2,
                )
                logger.info("Starting chat.....")
                self.chat = self.client.chats.create(model=self.model, config=config)
            
            response = self.chat.send_message(prompt)
            response_text = getattr(response, "text", str(response))

            memory.add({"role":"user", "text":prompt})
            memory.add({"role":"assistant", "text":response_text})

            self.speak(response_text)
            return response_text
        except Exception as e:
            logger.exception("Agent execution failed")
            return f"Agent FailureL {str(e)}"


