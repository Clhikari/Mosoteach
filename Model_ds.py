import traceback
import dashscope
import json

class ModelReasoning:
    def __init__(self):
        self.data = None
        key_path = "./user_data.json"
        self.Api_key = ""
        with open(key_path, 'r', encoding='utf-8') as f:
            self.Api_key = json.load(f)["user_data"]["Model_ds"]
        
    def set_data(self, data):
        self.data = data
        
    def deal_with(self):
        if not self.data:
            print("没有数据需要处理")
            return
            
        # 设置API密钥
        dashscope.api_key = self.Api_key
        
        # 构建Prompt
        prompt = f"""你回答题目时，只需要回答选项如:A,B,C这样,不需要其他的文字,还有单选就是单选,除非题目有说明多选,否则只给一个答案,还有些题型需要文字。下面是一道题目:{self.data}"""
        
        try:
            response = dashscope.Generation.call(
                model="deepseek-v3", 
                prompt=prompt, 
                max_tokens=1314
            )

            if response.status_code == 200:
                if response.output and 'choices' in response.output:
                    generated_text = response.output['choices'][0]['message']['content']
                    print("====题目答案====")
                    print(generated_text)
                    return generated_text
                else:
                    print("模型未生成有效输出")
            else:
                print(f"请求失败，状态码：{response.status_code}")
                
        except Exception as e:
            print(f"发生异常: {e}")
            traceback.print_exc()

if __name__ == "__main__":
    m = ModelReasoning()