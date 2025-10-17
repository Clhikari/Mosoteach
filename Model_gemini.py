from openai import OpenAI
from rich.console import Console
import traceback
import json

class Model_gemini:
    def __init__(self):
        self.data = None
        self.console = Console()
        key_path = "yunbaike_dome/user_data.json"
        self.Api_key = ""
        with open(key_path, 'r', encoding='utf-8') as f:
            self.Api_key = json.load(f)["user_data"]["Model_gemini"]
            
    def set_data(self, data):
        self.data = data
        
    def deal_with(self):
        if not self.data:
            self.console.print("没有数据需要处理")
            return
            
        # 构建客户端
        client = OpenAI(
            api_key="",
            base_url="https://generativelanguage.googleapis.com/v1beta/openai/"
        )

        try:
            response = client.chat.completions.create(
            model="gemini-2.5-flash",
            reasoning_effort="low",
            messages=[
                {"role": "system", "content": "你深知各种知识,回答题目时，只需要回答选项如:A,B,C这样,不需要其他的文字,还有单选就是单选,除非题目有说明多选,否则只给个答案,可能有些题型是填空题"},
                {
                    "role": "user",
                    "content": f"下面是一道题目:{self.data}"
                }
            ]
        )
            if response.choices[0].message:
                self.console.print("====题目答案====")
                self.console.print(response.choices[0].message.content)
                return response.choices[0].message.content 
        except Exception as e:
            self.console.print(f"发生异常: {e}")
            traceback.print_exc()