import json
import requests
import time
from lxml import etree
import os
import re
import random
from selenium.webdriver.chrome.options import Options
from selenium.webdriver.common.by import By
from selenium.webdriver.chrome.service import Service
from selenium import webdriver
from rich.console import Console
class yun_ban_ke:
    def __init__(self):
        self.url = "https://www.mosoteach.cn/web/index.php?c=clazzcourse&m=index"
        # self.proxies =  {
        #                     'http': 'socks5h://127.0.0.1:7897', 可选择用代理
        #                     'https': 'socks5h://127.0.0.1:7897'
        #                 }
        self.file_path = "./yunbaike_dome/user_data.json"
        with open(self.file_path, 'r', encoding='utf-8') as f:
                self.json_date = json.load(f)
        self.user_data = {
                "account" : self.json_date["user_data"]["user_name"],
                "ciphertext": self.json_date["user_data"]["password_encryption"]
            }
        self.Cookie = self.json_date["user_data"]["Cookie"]
        self.session = requests.Session()
        self.session.headers.update({
            "cookie" : self.Cookie,
            'User-Agent': 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/136.0.0.0 Safari/537.36'
        })
        self.chrome_options = Options()
        self.chrome_options.add_argument("user-agent=Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36(KHTML, like ) Chrome/136.0.self.0.0 Safari/537.36")
        # chrome_options.add_argument('--headless=new')  # 使用新的无头模式
        self.chrome_options.add_argument('--disable-gpu')
        self.chrome_options.add_argument('--disable-dev-shm-usage') # 克服资源限制问题
        self.chrome_options.add_experimental_option('excludeSwitches', ['enable-automation'])
        self.chrome_options.add_experimental_option('useAutomationExtension', False)
        self.interactionUrl = "https://www.mosoteach.cn/web/index.php?c=interaction&m=index"
        self.pre = "https://www.mosoteach.cn/web/index.php?c=interaction_quiz&m=start_quiz_confirm&clazz_course_id="
        self.path = "./yunbaike_dome/url_test"
        self.url_text = "./yunbaike_dome/url_test/url.txt"
        self.topic_url = set() # 存储文件里的题目
        self.test_list = [] # 存储正在处理的题目
        self.url_list = [] # 存储测试题的url
        self.id = []
        self.CHROME_DRIVER_PATH = "YOU_PATH_"
        if not os.path.exists(self.path):
            os.makedirs(self.path,exist_ok=True)
        try:
            with open(self.url_text,'r',encoding='utf-8') as f:
                for url in f.readlines():
                    self.topic_url.add(url.strip())
        except FileNotFoundError as e:
            self.console.print('路径不存在，表示为第一次开始做题',{e})
        
        self.console = Console()
        
    def driver_run(self):
        service = Service(executable_path=self.CHROME_DRIVER_PATH)
        driver = webdriver.Chrome(service=service, options=self.chrome_options)
        driver.get(self.url)
        time.sleep(5)
        try:
            user_name = driver.find_element(By.ID,"account-name")
            password = driver.find_element(By.ID,"user-pwd")
            user_name.send_keys(self.user_data['account'])
            time.sleep(1)
            password.send_keys(self.user_data['password'])
            time.sleep(2)
            son_1 = driver.find_element(By.ID,"login-button-1")
            son_1.click()
            time.sleep(10) # 等待登录跳转
            Cookies_list = driver.get_cookies()
            cookie_string = "; ".join([f"{cookie['name']}={cookie['value']}" for cookie in Cookies_list])
            return cookie_string
        except Exception as e_login:
            self.console.print(f"登录过程中发生错误: {e_login}")
            return None
        finally:
            driver.quit()
        
        # 请求方法
    def session_url(self):
        Referer = "https://www.mosoteach.cn/web/index.php?c=passport"
        html_data = self.session.post(url=self.url,headers={"Referer":Referer},timeout=15,data=self.user_data)
        self.user_data['password'] = self.json_date["user_data"]['password']
        html_data.raise_for_status()
        html_data.encoding = html_data.apparent_encoding
        self.console.print(html_data.text)
        str_data = html_data.text
        text_to_find = "sort-class-info"
        escaped_pattern = re.escape(text_to_find)
        find_data = re.findall(escaped_pattern, str_data,re.S)
        if find_data and len(find_data) == 0:
            self.console.print("Cookie已过期或为空")
            cookie_string = self.driver_run()
            if cookie_string:
                    self.session.headers.update({"cookie": cookie_string})
                    # 重新发起请求
                    html_data = self.session.post(url=self.url, headers={"Referer": Referer}, timeout=15, data=self.user_data)
                    html_data.raise_for_status()
                    html_data.encoding = html_data.apparent_encoding
                    # 读取文件
                    with open(self.file_path, 'r', encoding='UTF-8') as f:
                        date = json.load(f)
                    # 把新的Cookie写入到文件
                    date["user_data"]["Cookie"] = cookie_string
                    with open(self.file_path, 'w', encoding='UTF-8') as f:
                        json.dump(date, f, ensure_ascii=False, indent=4)
                    self.console.print(html_data.text)
            else:
                self.console.print("Driver未能成功获取Cookie")
                return None
        et = etree.HTML(html_data.text)
        return et
    
    
    
    # 处理课程名称
    def course_name_data(self):
        et = self.session_url()
        name_list = [] # 存储所有课程名称
        course_name = et.xpath('//div[@class="sort-class-info"]')
        if course_name != []:
            for name in course_name: # type: ignore
                text = name.xpath('./span[1]/text()')
                name_list.append(text[0])
        return name_list
        
    # 处理链接
    def link_data(self):
        et = self.session_url()
        all_piece = et.xpath('//section[@class="pop-sort pop-sort-class"]/ul/li') # 定位所有课程的id
        # self.console.print(all_piece)
        # input()
        link_list = [] # 存储所有课程的链接
        if all_piece != []:
            for id in all_piece:
                id = id.get('data-id')
                self.id.append(id)
                url = self.interactionUrl + "&clazz_course_id=" + id
                link_list.append(url)
        return link_list
    
    # 进入每个课程
    def son_link_(self):
        link_list = self.link_data()
        Referer = "https://www.mosoteach.cn/"
        for i,link in enumerate(link_list):
            time.sleep(random.uniform(1,3))
            son_html_data = self.session.get(url=link,headers={"Referer":Referer},timeout=15)
            son_html_data.raise_for_status()
            son_html_data.encoding = son_html_data.apparent_encoding
            self.son_deal_with(son_html_data,link,i) # 对每个子链接进行处理
    
    # 对课程页面进行处理
    def son_deal_with(self,son_html_data,link,i):
        # self.console.print(son_html_data.text)
        et =etree.HTML(son_html_data.text)
        activity = et.xpath('//div[@class="interaction-rows"]/div') # 获得所有的活动
        number = et.xpath('//*[@id="0"]//div/div[1]/div[3]/div[1]/span[3]') # 判断是否是题目
        # self.console.print(number)
        if activity == [] or number == []:
            return
        
        for line,num in zip(activity,number):
            task = line.xpath('.//span[@class="interaction-status processing"]')
            text = num.xpath('./text()')
            if task == [] or "题目" not in text[0]:  # 如果列表为空，活动已结束或者不是测试题
                continue
            id_ = line.get('data-id') # 获得测试题的id
            id_data = self.id[i]
            url = self.pre  + id_data + "&id=" + id_ + "&order_item=group"
            self.test_list.append(url)
            self.console.print(url)
    
    # 进入每道题目的url
    def test_data(self):
        self.son_link_()
        T = False
        for index,url_not_done in enumerate(self.test_list):
            time.sleep(random.uniform(1,3))
            html_data = self.session.get(url=url_not_done,timeout=15)
            html_data.raise_for_status()
            html_data.encoding = html_data.apparent_encoding
            # self.console.print(html_data.text)
            et = etree.HTML(html_data.text)
            html_text = et.xpath('//div[@class="hidden-box hidden-url"]/text()')
            if html_text == []:
                url = et.xpath('//div[@class="can-operate-color"]/a/@href')
                html_son = self.session.get(url=url,timeout=15)
                html_son.raise_for_status()
                html_son.encoding = html_son.apparent_encoding
                self.console.print(html_son)
                et = etree.HTML(html_son.text)
                html_text = et.xpath('//div[@class="hidden-box hidden-url"]/text()')
                T = self.url_pa(html_text)
                if T is True:
                    T = False
                    continue
                self.url_list.append(html_text)
            T = self.url_pa(html_text)
            if T is True:
                T = False
                continue
            self.console.print(html_text)
            self.url_list.append(html_text)
            # return self.url_list
        # self.data_parse()
        
    def url_pa(self,html_text):
        if not html_text or not html_text[0]:
            return False
        return html_text[0] in self.topic_url
