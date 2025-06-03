import requests
import time
from lxml import etree
import os
import random
import json

class yun_ban_ke:
    def __init__(self):
        self.url = "https://www.mosoteach.cn/web/index.php?c=clazzcourse&m=index"
        self.proxies =  {
                            'http': 'socks5h://127.0.0.1:7897',
                            'https': 'socks5h://127.0.0.1:7897'
                        }
        self.session = requests.Session()
        self.session.headers.update({
            "cookie" : "_uab_collina=174728336884969406676243; b-user-id=10ca73a3-efcc-e59a-eeb1-4493c14b3bf4; acw_tc=0a0966d617478890811671009e2b59c212232a84bc2c9668b527602aa1ac9c; teachweb=687aae30819fe7a5e5ba0f8af118c021e45925f0; SERVERID=8225607901597b15a5eced5225327c48|1747889093|1747889081; tfstk=gBHmLpbZWjPXIrT8wYwXtMF_wOO8h-w_JVBTWRUwazz5kZBxbA0ib46tMITfjb0Zyr7vcCgkI40EMNnaBGmahYhABrLbIcuLIeLpppnjcWwwJeLYkcUQQlJT7QFqYqyhheLppLQbOKWHJVnZzJtufzr4bOywq8rTYRrN3VzzakrduRuZ7zzzDksV0PWVz_z7bRzZ7R-uUzZgQPljg1anl2HyPagvtDv6CYq0iyXTrOXMXoV08mzk8ekkdS4E0zXwuqlebP4SLUdoVVlqkugMKU2EO4DgTRvcRocrx8zYLBfun03jK-kHotE8srcr37jNQ8y8S2hioEfbuj3uOuPPjpZ-Jzoj3bx1zmuLo-rzwp-n45l-hWMvzt2EOmex_28RMrl3bg8Ra6PHJOZyXY511Sr7qyF4IXWv0QZHg3xlOLP4Vo9kq3f11Sr7qyKkq69zguZXE",
            'User-Agent': 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/136.0.0.0 Safari/537.36'
        })
        self.interactionUrl = "https://www.mosoteach.cn/web/index.php?c=interaction&m=index"
        self.pre = "https://www.mosoteach.cn/web/index.php?c=interaction_quiz&m=start_quiz_confirm&clazz_course_id="
        self.path = "url_test"
        self.url_text = "url_test/url.txt"
        self.topic_url = set() # 存储文件里的题目
        self.test_list = [] # 存储正在处理的题目
        self.url_list = [] # 存储测试题的url
        self.id = []
        if not os.path.exists(self.path):
            os.makedirs(self.path,exist_ok=True)
        try:
            with open(self.url_text,'r',encoding='utf-8') as f:
                for url in f.readlines():
                    self.topic_url.add(url.strip())
        except FileNotFoundError as e:
            print('路径不存在，表示为第一次开始做题',{e})
        with open("user_date.json", 'r', encoding='utf-8') as f:
                json_date = json.load(f)
        self.data = {
                "account" : json_date["user_date"]["user_name"],
                "ciphertext": json_date["user_date"]["password"]
            }            
    
        # 请求方法
    def session_url(self):
        Referer = "https://www.mosoteach.cn/web/index.php?c=passport"
        html_data = self.session.post(url=self.url,headers={"Referer":Referer},proxies=self.proxies,timeout=15,data=self.data)
        html_data.raise_for_status()
        html_data.encoding = html_data.apparent_encoding
        print(html_data.text)
        et = etree.HTML(html_data.text)
        return et
    
    # 处理课程名称
    def course_name_data(self):
        et = self.session_url()
        name_list = [] # 存储所有课程名称
        course_name = et.xpath('//div[@class="sort-class-info"]')
        if course_name != []:
            for name in course_name:
                text = name.xpath('./span[1]/text()')
                name_list.append(text[0])
        return name_list
        
    # 处理链接
    def link_data(self):
        et = self.session_url()
        all_piece = et.xpath('//section[@class="pop-sort pop-sort-class"]/ul/li') # 定位所有课程的id
        # print(all_piece)
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
            son_html_data = self.session.get(url=link,headers={"Referer":Referer},proxies=self.proxies,timeout=15)
            son_html_data.raise_for_status()
            son_html_data.encoding = son_html_data.apparent_encoding
            self.son_deal_with(son_html_data,link,i) # 对每个子链接进行处理
    
    # 对课程页面进行处理
    def son_deal_with(self,son_html_data,link,i):
        # print(son_html_data.text)
        et =etree.HTML(son_html_data.text)
        activity = et.xpath('//div[@class="interaction-rows"]/div') # 获得所有的活动
        number = et.xpath('//*[@id="0"]//div/div[1]/div[3]/div[1]/span[3]') # 判断是否是题目
        # print(number)
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
            print(url)
    
    # 进入每道题目的url
    def test_data(self):
        self.son_link_()
        T = False
        for index,url_not_done in enumerate(self.test_list):
            time.sleep(random.uniform(1,3))
            html_data = self.session.get(url=url_not_done,timeout=15,proxies=self.proxies)
            html_data.raise_for_status()
            html_data.encoding = html_data.apparent_encoding
            # print(html_data.text)
            et = etree.HTML(html_data.text)
            html_text = et.xpath('//div[@class="hidden-box hidden-url"]/text()')
            if html_text == []:
                url = et.xpath('//div[@class="can-operate-color"]/a/@href')
                html_son = self.session.get(url=url,proxies=self.proxies,timeout=15)
                html_son.raise_for_status()
                html_son.encoding = html_son.apparent_encoding
                print(html_son)
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
            print(html_text)
            self.url_list.append(html_text)
            # return self.url_list
        # self.data_parse()
        
    def url_pa(self,html_text):
        if not html_text or not html_text[0]:
            return False
        return html_text[0] in self.topic_url
