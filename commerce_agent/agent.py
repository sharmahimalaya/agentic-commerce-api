import sys
import logging
import os
from google import genai
from google.genai import types
from .config import settings
from .tools import list_products, create_cart_and_add_items, checkout_and_pay_cart
from . import memory

# Basic logger so we can see what the agent is thinking/doing in the terminal
try:
    import pyttsx3
except Exception:
    # TTS is optional, so if loading fails (e.g. on Linux/macOS without sound libraries), we just skip it
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
        # Double check if API key exists, print a helpful warning if not
        if not settings.gemini_api_key and not os.environ.get("GEMINI_API_KEY"):
            logger.warning("No GEMINI_API_KEY defined. Please set it in system environment or .env")
        
        # Initialize Google GenAI client
        self.client = genai.Client(api_key=settings.gemini_api_key.get_secret_value() if settings.gemini_api_key else None)
        self.model = settings.gemini_model
        self.chat = None

        # Give the LLM access to these python functions. Gemini will call them automatically when needed!
        self.tools = [list_products, create_cart_and_add_items, checkout_and_pay_cart]

        # The system instructions tell Gemini how to behave and what steps to follow to purchase items safely.
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

        # Initialize TTS (Text-to-Speech) engine if it's installed
        self.tts_engine = None
        if pyttsx3 is not None:
            try:
                self.tts_engine = pyttsx3.init()
            except Exception:
                self.tts_engine = None

    # Speak helper for talking responses aloud (TTS)
    def speak(self, text: str) -> None:
        if not self.tts_engine:
            return
        try:
            self.tts_engine.say(text)
            self.tts_engine.runAndWait()
        except Exception:
            # Just ignore speech errors so the agent doesn't crash if audio is busy
            pass

    def execute_task(self, prompt: str) -> str:
        """
        Runs a ReAct execution loop, automatically calling tools and
        converging on the final user solution.
        Stores brief memory and can speak responses aloud.
        """
        logger.info(f"Received user prompt: {prompt}")
        try:
            # If this is the start of a conversation, set up the chat session
            if self.chat is None:
                # Load recent chat history to give the LLM context
                recent = memory.recent(5)
                memory_section = "\n".join(recent) if recent else ""
                system_instruction = self.system_instruction_base + ("\n\nRecent memory: \n" + memory_section if memory_section else "")
                config = types.GenerateContentConfig(
                    system_instruction=system_instruction,
                    tools=self.tools,
                    temperature = 0.2, # Keep temperature low so it is deterministic and uses tools correctly
                )
                logger.info("Starting chat.....")
                self.chat = self.client.chats.create(model=self.model, config=config)
            
            # Send message to Gemini chat session
            response = self.chat.send_message(prompt)
            response_text = getattr(response, "text", str(response))

            # Remember this turn
            memory.add({"role":"user", "text":prompt})
            memory.add({"role":"assistant", "text":response_text})

            # Read out loud if TTS is ready
            self.speak(response_text)
            return response_text
        except Exception as e:
            logger.exception("Agent execution failed")
            return f"Agent Failure {str(e)}"



