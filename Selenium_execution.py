from selenium import webdriver
from selenium.webdriver.common.by import By
from selenium.webdriver.support.ui import WebDriverWait
from selenium.webdriver.support import expected_conditions as EC
from selenium.common.exceptions import TimeoutException, ElementNotInteractableException
from selenium.webdriver.chrome.service import Service
from data_processing import data_processing
import time
from lxml import etree
from Model_reasoning import ModelReasoning
from Model_gemini import Model_gemini
from collections import defaultdict
import os
import re
import platform

class selenium_execution(data_processing):
    def __init__(self):
        super().__init__()
        self.data = None
        self.model = ModelReasoning()
        self.model_gemini = Model_gemini()
        self.driver = None
        self.topic_count = 0
        self.url_count = 0
        self.out_count = None
        self.path = "url_test"
        self.QUESTION_TYPES = {
            'fill': {'填空题', '填空'},
            'single': {'单选题'},
            'multiple': {'多选题'},
            'choice': {'单选题', '多选题'}
        }
        
    def data_processing(self, all_text, et, quiz_page_url_data):
        data = et.xpath('//div[@class="t-subject t-item moso-text moso-editor"]')
        out_text = self.driver.find_elements(By.XPATH, './/div[@class="tp-blank t-item"]')
        self.out_count = 0
        option_type = et.xpath('//div[@class="t-con"]/div/div[@class="t-type SINGLE"]|//div[@class="t-con"]/div/div[@class="t-type MULTI"]|//div[@class="t-con"]/div/div[@class="t-type FILL"]')
        option_labels_selenium = self.driver.find_elements(By.XPATH, './/div[contains(@class, "t-subject t-item moso-text moso-editor")]')
        self.topic_count = 0
        pattern = r'^\d+\.'
        
        for line, lalen, option, text_type in zip(all_text, data, option_labels_selenium, option_type):
            text_type = text_type.xpath('./text()')[0]
            question_stem = line.xpath('.//text()')
            question_stem = "".join([text.strip() for text in question_stem if text.strip()])
            self.console.print(question_stem)
            
            lalen_list = lalen.xpath('./following-sibling::div[contains(@class, "t-option") and contains(@class, "t-item")]')
            if lalen_list != []:
                lalen_list = lalen_list[0]
                lalen_list = lalen_list.xpath('.//label[contains(@class, "el-radio")]')
            
            self.data = ''
            lines = []
            current_line_parts = []
            
            if not re.search(pattern, question_stem) and text_type in self.QUESTION_TYPES['fill']:
                self.data += self.fill_in_the_blank(line, lines, current_line_parts, question_stem)
            elif not re.search(pattern, question_stem) and text_type in self.QUESTION_TYPES['choice']:
                self.data += self.fill_in_the_blank(line, lines, current_line_parts, question_stem)
                self.for_index(lalen_list)
            elif re.search(pattern, question_stem) and text_type in self.QUESTION_TYPES['fill']:
                self.data += self.fill_in_the_blank(line, lines, current_line_parts, question_stem)
            else:
                self.data += f"""{question_stem}"""
                self.for_index(lalen_list)
            
            self.console.print(text_type)
            self.model.set_data(text_type + '\n' + self.data)
            time.sleep(0.2)
            answer = self.model.deal_with()
            
            if len(answer) != 1 and text_type in self.QUESTION_TYPES['choice']:
                answer = answer.split(',')
            
            self.Click(answer, option, text_type, out_text)
            time.sleep(1)
            self.topic_count += 1
            self.console.print(f"---已完成第{self.topic_count}题---")
        
        self.submit_answer(quiz_page_url_data)

    def for_index(self, lalen_list):
        for number in lalen_list:
            num_1 = number.xpath('./span[2]/div/span[1]/text()')[0]
            num_2 = number.xpath('./span[2]/div/span[2]/text()')[0]
            self.data += f"""\n{num_1.strip()}{num_2.strip()}"""
            self.console.print(num_1, num_2)
        
    def fill_in_the_blank(self, line, lines, current_line_parts, question_stem):
        for br in line.xpath('child::node()'):
            if hasattr(br, 'tag') and br.tag == 'br':
                if current_line_parts:
                    lines.append(''.join(current_line_parts).strip())
                current_line_parts = []
            elif isinstance(br, etree._ElementUnicodeResult) or isinstance(br, str):
                text_content = str(br)
                current_line_parts.append(text_content)
        
        if current_line_parts:
            lines.append("".join(current_line_parts).strip())
        
        for text in lines:
            if question_stem not in text:
                question_stem += text
        
        return f"{question_stem}"
    
    def submit_answer(self, quiz_page_url_data):
        try:
            son = self.driver.find_element(By.XPATH, '//*[@id="app"]/div/div[2]/div[2]/div[3]/button')
            time.sleep(1)
            son.click()
            button = son.find_element(By.XPATH, '/html/body/div[15]/div/div[3]/button')
            time.sleep(0.5)
            button.click()
            son = son.find_element(By.XPATH, '/html/body/div[15]/div/div[3]/button')
            clickable_element = WebDriverWait(self.driver, 10).until(
                EC.element_to_be_clickable(son)
            )
            clickable_element.click()
            time.sleep(3)
            
            url = "url.txt"
            path = os.path.join(self.path, url)
            with open(path, 'a', encoding='utf-8') as f:
                f.write("\n" + quiz_page_url_data[0])
                self.console.print(f"已将{quiz_page_url_data[0]}写入到文件中")
            time.sleep(5)
            
        except ElementNotInteractableException as e:
            self.console.print(f"不可交互'{e}',使用js")
            element_for_js_click = self.driver.find_element(*son)
            self.driver.execute_script("arguments[0].click();", element_for_js_click)
            self.console.print("已使用 JavaScript 点击。")
            url = "url.txt"
            path = os.path.join(self.path, url)
            with open(path, 'a', encoding='utf-8') as f:
                f.write(quiz_page_url_data[0] + "\n")
            time.sleep(5)

    def Click(self, answer, option, text_type, text_out):
        if text_type in self.QUESTION_TYPES['choice']:
            self.answer_count = 0
            option = option.find_element(By.XPATH, './following-sibling::div[contains(@class, "t-option") and contains(@class, "t-item")]')
            label_list = option.find_elements(By.XPATH, './/label[contains(@class, "el-radio")]|.//label[contains(@class, "el-checkbox")]')
            cut = defaultdict(int)
            
            for index in label_list:
                letter = index.find_elements(By.XPATH, './span[2]/div/span[1]')
                letter = letter[0].text.strip()
                letter = letter.replace('.', '').replace(' ', '')
                
                if answer == letter and len(answer) == 1:
                    self.driver.execute_script("arguments[0].scrollIntoView({block: 'center', inline: 'nearest'});", index)
                    time.sleep(0.3)
                    all_option_labels_selenium = WebDriverWait(self.driver, 5).until(
                        EC.element_to_be_clickable(index)
                    )
                    self.driver.execute_script("arguments[0].click();", all_option_labels_selenium)
                    break
                
                elif len(answer) != 1:
                    for line in answer:
                        if line == letter and cut[line] < 1:
                            self.driver.execute_script("arguments[0].scrollIntoView({block: 'center', inline: 'nearest'});", index)
                            time.sleep(0.3)
                            all_option_labels_selenium = WebDriverWait(self.driver, 5).until(
                                EC.element_to_be_clickable(index)
                            )
                            all_option_labels_selenium.click()
                            cut[line] += 1
            return
        
        elif text_type in self.QUESTION_TYPES['fill']:
            input_text = text_out[self.out_count].find_element(By.XPATH, './/input[@class="el-input__inner"]')
            time.sleep(0.3)
            input_text.send_keys(answer)
            self.out_count += 1
                    
    def run(self):
        self.test_data()
        login_url = 'https://www.mosoteach.cn/web/index.php?c=passport&m=index'
        
        try:
            service = Service(executable_path=self.CHROME_DRIVER_PATH)
            self.driver = webdriver.Chrome(service=service, options=self.chrome_options)
        except Exception as e:
            self.console.print(f"[red]无法启动Chrome浏览器: {e}[/red]")
            if platform.system() == "Linux":
                self.console.print("[yellow]请确保已安装Chrome和ChromeDriver：")
                self.console.print("sudo apt install google-chrome-stable chromium-chromedriver[/yellow]")
            return
        
        self.driver.get(url=login_url)
        time.sleep(3)
        
        try:
            user_name = self.driver.find_element(By.ID, "account-name")
            password = self.driver.find_element(By.ID, "user-pwd")
            user_name.send_keys(self.user_data['account'])
            time.sleep(1)
            password.send_keys(self.user_data['password'])
            time.sleep(2)
            son_1 = self.driver.find_element(By.ID, "login-button-1")
            son_1.click()
            time.sleep(10)
        except Exception as e_login:
            self.console.print(f"登录过程中发生错误: {e_login}")
            self.driver.quit()
            return
        
        self.url_count = len(self.url_list)
        if len(self.url_list) == 0:
            self.console.print("----------------------当前已无测试题可做------------------------")
            return
        
        self.console.print(f"------------------共有{self.url_count}个测试题--------------------")
        url_list = self.url_list
        
        for index, quiz_page_url_data in enumerate(url_list):
            self.console.print(f"-----------------------------正在处理第{index + 1}个,还剩{self.url_count - index - 1}个测试题待处理---------------------------")
            current_quiz_url = quiz_page_url_data[0]
            self.driver.get(url=current_quiz_url)
            time.sleep(5)
            
            try:
                WebDriverWait(self.driver, 20).until(
                    EC.presence_of_element_located((By.XPATH, '//div[@class="con-list"]'))
                )
            except TimeoutException:
                self.console.print(f"等待题目容器加载超时:{current_quiz_url}")
                continue
            
            html_dome = self.driver.page_source
            et = etree.HTML(html_dome)
            all_question_stem_elements = et.xpath('//div[@class="t-subject t-item moso-text moso-editor"]')
            self.console.print(f"找到 {len(all_question_stem_elements)} 个题干元素。")
            
            if all_question_stem_elements:
                self.data_processing(all_question_stem_elements, et, quiz_page_url_data)
            else:
                self.console.print("当前页面未找到题干元素。")
            
            time.sleep(2)

if __name__ == "__main__":
    selenium = selenium_execution()
    selenium.run()
    if selenium.driver:
        selenium.driver.quit()