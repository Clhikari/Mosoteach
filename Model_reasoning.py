import traceback
import dashscope
from rich.console import Console

class ModelReasoning:
    def __init__(self):
        self.data = None
        self.console = Console()
    def set_data(self, data):
        self.data = data
        
    def deal_with(self):
        if not self.data:
            self.console.print("没有数据需要处理")
            return
            
        # 设置API密钥
        dashscope.api_key = "sk-7938f4fb83e849c49f6c1e722274cd7b"
        
        # 构建Prompt
        prompt = f"""你深知各种知识,回答题目时，只需要回答选项如:A,B,C这样,不需要其他的文字,还有单选就是单选,除非题目有说明多选,否则只给个答案,可能有些题型是填空题。下面是一道题目:{self.data}"""
        
        try:
            response = dashscope.Generation.call(
                model="deepseek-v3", 
                prompt=prompt, 
                max_tokens=114514
            )

            if response.status_code == 200: # type: ignore
                if response.output and 'choices' in response.output: # type: ignore
                    generated_text = response.output['choices'][0]['message']['content'] # type: ignore
                    self.console.print("====题目答案====")
                    self.console.print(generated_text)
                    return generated_text
                else:
                    self.console.print("模型未生成有效输出")
            else:
                self.console.print(f"请求失败，状态码：{response.status_code}") # type: ignore
                
        except Exception as e:
            self.console.print(f"发生异常: {e}")
            traceback.print_exc()
