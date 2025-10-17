import json
import requests
import time
from lxml import etree
import os
import re
import random
import platform
from selenium.webdriver.chrome.options import Options
from selenium.webdriver.common.by import By
from selenium.webdriver.chrome.service import Service
from selenium import webdriver
from rich.console import Console

class data_processing:
    def __init__(self):
        self.url = "https://www.mosoteach.cn/web/index.php?c=clazzcourse&m=index"
        # self.proxies =  {
        #                     'http': 'socks5h://127.0.0.1:7897',
        #                     'https': 'socks5h://127.0.0.1:7897'
        #                 }
        
        # 检测操作系统并设置路径
        self.console = Console()
        self.system = platform.system()
        if self.system == "Linux":
            self.file_path = "./yunbaike_dome/user_data.json"
            self.path = "./url_test"
            self.url_text = "./url_test/url.txt"
            # Linux自动检测ChromeDriver路径
            self.CHROME_DRIVER_PATH = self._find_chromedriver()
        else:
            # Windows路径
            self.file_path = "./yunbaike_dome/user_data.json"
            self.path = "./yunbaike_dome/url_test"
            self.url_text = "./yunbaike_dome/url_test/url.txt"
            self.CHROME_DRIVER_PATH = "D:\\aobo\\automation\\chromedriver-win64\\chromedriver.exe"
            
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
        
        # Chrome选项配置
        self.chrome_options = Options()
        self.chrome_options.add_argument("user-agent=Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36(KHTML, like ) Chrome/136.0.0.0 Safari/537.36")
        
        # Linux服务器必须使用无头模式
        if self.system == "Linux":
            # 自动检测Chrome/Chromium二进制文件
            chrome_binary = self._find_chrome_binary()
            if chrome_binary:
                self.chrome_options.binary_location = chrome_binary
                self.console.print(f"[green]使用浏览器: {chrome_binary}[/green]")
            
            self.chrome_options.add_argument('--headless=new')
            self.chrome_options.add_argument('--no-sandbox')  # Linux必需
            self.chrome_options.add_argument('--disable-dev-shm-usage')
            self.chrome_options.add_argument('--disable-gpu')
            self.chrome_options.add_argument('--disable-software-rasterizer')
            self.chrome_options.add_argument('--remote-debugging-port=9222')
        else:
            # Windows可选无头模式
            # self.chrome_options.add_argument('--headless=new')
            self.chrome_options.add_argument('--disable-gpu')
            self.chrome_options.add_argument('--disable-dev-shm-usage')
            self.chrome_options.binary_location = "D:\\aobo\\automation\\chrome-win64\\chrome.exe"
        self.chrome_options.add_experimental_option('excludeSwitches', ['enable-automation'])
        self.chrome_options.add_experimental_option('useAutomationExtension', False)
        
        self.interactionUrl = "https://www.mosoteach.cn/web/index.php?c=interaction&m=index"
        self.pre = "https://www.mosoteach.cn/web/index.php?c=interaction_quiz&m=start_quiz_confirm&clazz_course_id="
        self.topic_url = set()
        self.test_list = []
        self.url_list = []
        self.id = []
        self._course_resources_list = []
        if not os.path.exists(self.path):
            os.makedirs(self.path, exist_ok=True)
        try:
            with open(self.url_text, 'r', encoding='utf-8') as f:
                for url in f.readlines():
                    self.topic_url.add(url.strip())
        except FileNotFoundError as e:
            pass
    
    def _find_chromedriver(self):
        """自动查找ChromeDriver路径"""
        possible_paths = [
            "/usr/bin/chromedriver",
            "/usr/bin/chromium-chromedriver",
            "/usr/local/bin/chromedriver",
            "/snap/bin/chromium.chromedriver",
            "/usr/lib/chromium-browser/chromedriver"
        ]
        
        for path in possible_paths:
            if os.path.exists(path):
                self.console.print(f"[green]找到ChromeDriver: {path}[/green]")
                return path
        
        self.console.print("[yellow]警告: 未找到ChromeDriver，使用默认路径[/yellow]")
        return "/usr/bin/chromedriver"
    
    def _find_chrome_binary(self):
        """自动查找Chrome/Chromium二进制文件"""
        possible_binaries = [
            "/usr/bin/google-chrome",
            "/usr/bin/google-chrome-stable",
            "/usr/bin/chromium-browser",
            "/usr/bin/chromium",
            "/snap/bin/chromium"
        ]
        
        for binary in possible_binaries:
            if os.path.exists(binary):
                return binary
        
        return None
        
    def get_resources(self):
        """返回课程资源列表。"""
        return self._course_resources_list
        
    def driver_run(self):
        try:
            service = Service(executable_path=self.CHROME_DRIVER_PATH)
            driver = webdriver.Chrome(service=service, options=self.chrome_options)
        except Exception as e:
            self.console.print(f"[red]ChromeDriver启动失败: {e}[/red]")
            self.console.print(f"[yellow]请确保已安装Chrome和ChromeDriver[/yellow]")
            return None
            
        driver.get(self.url)
        time.sleep(5)
        try:
            user_name = driver.find_element(By.ID, "account-name")
            password = driver.find_element(By.ID, "user-pwd")
            user_name.send_keys(self.user_data['account'])
            time.sleep(1)
            password.send_keys(self.user_data['password'])
            time.sleep(2)
            son_1 = driver.find_element(By.ID, "login-button-1")
            son_1.click()
            time.sleep(10)
            Cookies_list = driver.get_cookies()
            cookie_string = "; ".join([f"{cookie['name']}={cookie['value']}" for cookie in Cookies_list])
            return cookie_string
        except Exception as e_login:
            self.console.print(f"登录过程中发生错误: {e_login}")
            return None
        finally:
            driver.quit()
        
    def session_url(self):
        Referer = "https://www.mosoteach.cn/web/index.php?c=passport"
        self.user_data['password'] = self.json_date["user_data"]['password']
        html_data = self.session.post(url=self.url, headers={"Referer": Referer}, timeout=15, data=self.user_data)
        html_data.raise_for_status()
        html_data.encoding = html_data.apparent_encoding
        cookie_string = self.driver_run()
        if cookie_string:
            self.session.headers.update({"cookie": cookie_string})
            html_data = self.session.post(url=self.url, headers={"Referer": Referer}, timeout=15, data=self.user_data)
            html_data.raise_for_status()
            html_data.encoding = html_data.apparent_encoding
            with open(self.file_path, 'r', encoding='UTF-8') as f:
                date = json.load(f)
            date["user_data"]["Cookie"] = cookie_string
            with open(self.file_path, 'w', encoding='UTF-8') as f:
                json.dump(date, f, ensure_ascii=False, indent=4)
            # self.console.print(html_data.text)
        else:
            self.console.print("Driver未能成功获取Cookie")
            return None
        et = etree.HTML(html_data.text)
        return et
    
    def course_name_data(self):
        et = self.session_url()
        # self.console.print(et)
        name_list = []
        course_name = et.xpath('//div[@class="sort-class-info"]')
        if course_name != []:
            for name in course_name:
                text = name.xpath('./span[1]/text()')
                name_list.append(text[0])
        return name_list
    
    def course_resources(self, et):
        resources_list = et.xpath('//div[@id="menu-content-box"]/div/a/@href')
        
        if not resources_list:
            self.console.print('[yellow]当前课程页面未找到资源区链接。[/yellow]')
            return []
            
        resources_link = resources_list[0]
        
        pattern = r'clazz_course_id=([^&]*)'
        match = re.search(pattern, resources_link)
        if not match:
            self.console.print('[red]错误：无法从资源链接中解析出 course_id。[/red]')
            return []
        clazz_course_id = match.group(1)
        
        # 请求资源页面
        resources_response = self.session.get(url=resources_link, headers={"Referer": resources_link}, timeout=15)
        resources_response.raise_for_status()
        resources_response.encoding = resources_response.apparent_encoding
        
        # 解析资源页面
        et_res = etree.HTML(resources_response.text)
        
        # 判断是否有资源
        now_resources = et_res.xpath('//div[@id="cc-main"]//div[@id="res-list-box"]/div//p/text()')
        if now_resources:
            self.console.print('当前暂无课程资源')
            return [] # 返回空列表表示没有资源

        all_resources = et_res.xpath('//div[@id="cc-main"]//div[@id="res-list-box"]/div')
        all_courses_list = []
        for resources in all_resources:
            try:
                resources_group_name = resources.xpath('./div[1]//span[@class="res-group-name"]/text()')[0]
            except IndexError:
                continue

            course_item = {
                "课程名称": resources_group_name,
                "文件列表": {}
            }
            
            resources_file_list = resources.xpath('./div[2]/div')
            
            for resources_file in resources_file_list:
                try:
                    resources_file_data = resources_file.xpath('./div[4]/div/span/text()')
                    file_name = resources_file_data[0]
                    file_size = resources_file_data[3]
                    view_count = resources_file_data[9]
                    experience = resources_file_data[7]
                    resources_file_id = resources_file.xpath('./div[1]/@id')[0][4:]
                    download_url = f"https://www.mosoteach.cn/web/index.php?c=res&m=download&file_id={resources_file_id}&clazz_course_id={clazz_course_id}"
                    
                    file_details = {
                        "大小": file_size,
                        "查看人数": view_count,
                        "经验": experience,
                        "下载链接": download_url
                    }
                    
                    course_item["文件列表"][file_name] = file_details
                except IndexError:
                    pass
            
            if course_item["文件列表"]:
                all_courses_list.append(course_item)
        
        return all_courses_list # 将处理好的列表返回

    def get_course_interaction_page(self, course_link):
        """获取并解析单个课程的互动页面"""
        Referer = "https://www.mosoteach.cn/"
        try:
            son_html_data = self.session.get(url=course_link, headers={"Referer": Referer}, timeout=15)
            son_html_data.raise_for_status()
            son_html_data.encoding = son_html_data.apparent_encoding
            return etree.HTML(son_html_data.text)
        except Exception as e:
            self.console.print(f"[red]获取课程页面失败: {e}[/red]")
            return None

    def link_data(self):
        et = self.session_url()
        all_piece = et.xpath('//section[@class="pop-sort pop-sort-class"]/ul/li')
        link_list = []
        if all_piece != []:
            for id in all_piece:
                id = id.get('data-id')
                self.id.append(id)
                url = self.interactionUrl + "&clazz_course_id=" + id
                link_list.append(url)
        return link_list
    
    def son_link_(self):
        link_list = self.link_data()
        Referer = "https://www.mosoteach.cn/"
        for i, link in enumerate(link_list):
            time.sleep(random.uniform(1, 3))
            son_html_data = self.session.get(url=link, headers={"Referer": Referer},  timeout=15)
            son_html_data.raise_for_status()
            son_html_data.encoding = son_html_data.apparent_encoding
            self.son_deal_with(son_html_data, link, i)
    
    def son_deal_with(self, son_html_data, link, i):
        et = etree.HTML(son_html_data.text)
        self._course_resources_list.append(self.course_resources(et))
        activity = et.xpath('//div[@class="interaction-rows"]/div')
        number = et.xpath('//*[@id="0"]//div/div[1]/div[3]/div[1]/span[3]')
        if activity == [] or number == []:
            return
        
        for line, num in zip(activity, number):
            task = line.xpath('.//span[@class="interaction-status processing"]')
            text = num.xpath('./text()')
            if task == [] or "题目" not in text[0]:
                continue
            id_ = line.get('data-id')
            id_data = self.id[i]
            url = self.pre + id_data + "&id=" + id_ + "&order_item=group"
            self.test_list.append(url)
            # self.console.print(url)
    
    def test_data(self):
        self.son_link_()
        T = False
        for index, url_not_done in enumerate(self.test_list):
            time.sleep(random.uniform(1, 3))
            html_data = self.session.get(url=url_not_done, timeout=15)
            html_data.raise_for_status()
            html_data.encoding = html_data.apparent_encoding
            et = etree.HTML(html_data.text)
            html_text = et.xpath('//div[@class="hidden-box hidden-url"]/text()')
            if html_text == []:
                url = et.xpath('//div[@class="can-operate-color"]/a/@href')
                html_son = self.session.get(url=url, timeout=15)
                html_son.raise_for_status()
                html_son.encoding = html_son.apparent_encoding
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
            # self.console.print(html_text)
            self.url_list.append(html_text)
        
    def url_pa(self, html_text):
        if not html_text or not html_text[0]:
            return False
        return html_text[0] in self.topic_url
    
    