import sys
import os
from dotenv import load_dotenv

# Load local environment variables (.env) before importing modules
load_dotenv()

from .agent import CommerceAgent

def main():
    agent = CommerceAgent()

    if len(sys.argv) >= 2:
        task_prompt = sys.argv[1]
        print("\n--- Starting Commerce AI Agent (One-shot) ---")
        result = agent.execute_task(task_prompt)
        print("\n--- Agent Response ---")
        print(result)
        print("-----------------------\n")
        return
    
    print("\n==============================================")
    print("      Commerce AI Agent - Interactive Chat    ")
    print("   Type 'exit' or 'quit' to end the session.  ")
    print("==============================================\n")
    
    while True:
        try:
            user_input = input("You: ").strip()
            if not user_input:
                continue
            if user_input.lower() in ("exit", "quit"):
                print("Goodbye!")
                break
                
            result = agent.execute_task(user_input)
            print(f"\nAgent: {result}\n")
        except (KeyboardInterrupt, EOFError):
            print("\nGoodbye!")
            break

if __name__ == "__main__":
    main()
