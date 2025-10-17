import json
import os
import time
from rich.tree import Tree
from rich.console import Console
from rich.panel import Panel
from rich.text import Text
from rich.prompt import Prompt
from data_processing import data_processing
from Selenium_execution import selenium_execution

class app_main():
    def __init__(self) -> None:
        # 初始化 rich console
        self.console = Console()
        self.executor = data_processing()
        self.selenium = selenium_execution()
        
    def display_main_menu(self):
        """显示主菜单界面"""
        os.system("cls" if os.name == "nt" else "clear")
        
        menu_content = Text()

        ascii_art = """
    ██╗   ██╗███╗   ██╗██████╗  █████╗ ███╗   ██╗██╗  ██╗███████╗
    ██║   ██║████╗  ██║██╔══██╗██╔══██╗████╗  ██║██║ ██╔╝██╔════╝
    ██║   ██║██╔██╗ ██║██████╔╝███████║██╔██╗ ██║█████╔╝ █████╗  
    ╚██╗ ██╔╝██║╚██╗██║██╔══██╗██╔══██║██║╚██╗██║██╔═██╗ ██╔══╝  
        ╚████╔╝ ██║ ╚████║██║  ██║██║  ██║██║ ╚████║██║  ██╗███████╗
        ╚═══╝  ╚═╝  ╚═══╝╚═╝  ╚═╝╚═╝  ╚═╝╚═╝  ╚═══╝╚═╝  ╚═╝╚══════╝
        """
        menu_content.append(ascii_art, style="bold cyan")
        menu_content.append("\n" + " " * 55)
        menu_content.append("v1.1.0 by Gemini\n", style="dim")

        menu_content.append("─" * 66, style="bright_black")
        menu_content.append("\n\n")

        try:
            with open("./user_data.json", "r", encoding="utf-8") as f:
                user_data = json.load(f)
                username = user_data.get("user_data", {}).get("user_name", "未知")
                display_username = (
                    username[:3] + "****" + username[7:]
                    if len(username) == 11
                    else username
                )
        except Exception:
            display_username = "无法读取"

        menu_content.append(f"  当前用户: ", style="bright_blue")
        menu_content.append(f"{display_username}\n\n", style="bold yellow")

        menu_content.append("  请选择要执行的操作：\n\n", style="bold cyan")
        menu_content.append("    🚀 [1] 开始自动答题\n", style="bold green")
        menu_content.append("         (进入交互式选择或一键完成)\n\n", style="dim")
        menu_content.append("    📚 [2] 浏览课程资源\n", style="bold blue")
        menu_content.append("         (查看和下载课程文件)\n\n", style="dim")
        menu_content.append("    ⚙️  [3] 用户配置\n", style="bold yellow")
        menu_content.append("         (查看您的登录信息)\n\n", style="dim")
        menu_content.append("    🚪 [4] 退出程序\n", style="bold red")

        self.console.print(
            Panel(
                menu_content,
                title="[bold blue]云班课 (Mosoteach) 自动答题助手[/bold blue]",
                border_style="blue"
            )
        )


    def handle_auto_answer(self):
        """处理自动答题选项"""
        while True:
            os.system("cls" if os.name == "nt" else "clear")
            
            answer_menu = Text("请选择答题模式：\n\n", style="bold green")
            answer_menu.append("  [1] 一键完成所有未做题目\n", style="yellow")
            answer_menu.append("  [2] 交互式选择课程\n", style="yellow")
            answer_menu.append("  [b] 返回主菜单\n", style="dim")
            
            self.console.print(
                Panel(
                    answer_menu,
                    title="[bold blue]🚀 自动答题[/bold blue]",
                    border_style="blue"
                )
            )
            
            choice = Prompt.ask("👉 请输入选项")
            
            if choice == "1":
                self.console.print("\n[bold cyan]正在初始化答题系统...[/bold cyan]")
                self.selenium.run()
                self.console.print("\n[bold cyan]已全部完成题目[/bold cyan]")
                if self.selenium:
                    self.selenium.driver.quit()
            elif choice == "2":
                self.console.print("\n[bold cyan]正在初始化交互式答题...[/bold cyan]")
                time.sleep(1)
                self.console.print("[bold yellow]功能待实现 - 交互式模式[/bold yellow]")
                time.sleep(2)
            elif choice.lower() == "b":
                break
            else:
                self.console.print("[bold red]无效输入，请重试[/bold red]")
                time.sleep(1)


    def browse_resources_menu(self):
        """浏览所有课程资源"""
        self.console.print("\n[bold cyan]正在加载所有课程资源...[/bold cyan]")
        
        with self.console.status("[bold green]正在获取资源列表...[/bold green]"):
            self.executor.test_data()
            resources_data = self.executor.get_resources()
        
        if not resources_data:
            self.console.print(
                Panel(
                    "[yellow]🙁 当前没有找到任何资源文件。[/yellow]",
                    title="[bold blue]📚 课程资源[/bold blue]",
                )
            )
            time.sleep(2)
            return

        # 显示资源树
        os.system("cls" if os.name == "nt" else "clear")
        tree = Tree("📚 [bold green]所有课程资源[/bold green]", guide_style="bright_blue")
        
        has_content = False

        for inner_list in resources_data:
            
            # 确保内层也是一个列表
            if not isinstance(inner_list, list):
                continue
                
            for course_dict in inner_list:
                
                if not isinstance(course_dict, dict) or "课程名称" not in course_dict:
                    continue
                
                has_content = True

                group_branch = tree.add(f"📁 [cyan]{course_dict['课程名称']}[/cyan]")
                
                if "文件列表" in course_dict and course_dict["文件列表"]:
                    for file_name, details in course_dict["文件列表"].items():
                        file_branch = group_branch.add(f"📄 [yellow]{file_name}[/yellow]")
                        file_branch.add(f"💾 [dim]大小:[/dim] {details['大小']}")
                        file_branch.add(f"👀 [dim]查看:[/dim] {details['查看人数']}")
                        file_branch.add(f"✨ [dim]经验:[/dim] {details['经验']}")
                else:
                    group_branch.add("[dim]（该分组下无文件）[/dim]")

        # 4. 在所有循环结束后，如果一次有效内容都没找到，再显示提示
        if not has_content:
            input("\n[dim]按 Enter 键返回课程列表...[/dim]")
            return
            

        self.console.print(
            Panel(tree, title="[bold blue]课程资源详情[/bold blue]", border_style="blue")
        )
        input("\n[dim]按 Enter 键返回主菜单...[/dim]")



    def display_user_config(self):
        """显示用户配置信息"""
        os.system("cls" if os.name == "nt" else "clear")
        try:
            with open("./yunbaike_dome/user_data.json", "r", encoding="utf-8") as f:
                data = json.load(f).get("user_data", {})

            password = data.get("password", "N/A")
            masked_pass = (
                password[0] + "******" + password[-1] if len(password) > 2 else "******"
            )
            ds_key = data.get("Model_ds", "N/A")
            masked_ds_key = (
                ds_key[:5] + "****************" + ds_key[-4:] if ds_key != "N/A" else "N/A"
            )
            gemini_key = data.get("Model_gemini", "N/A")
            masked_gemini_key = (
                gemini_key[:5] + "****************" + gemini_key[-4:]
                if gemini_key != "N/A"
                else "N/A"
            )

            config_text = Text()
            config_text.append("用户名:         ", style="cyan")
            config_text.append(f"{data.get('user_name', 'N/A')}\n", style="yellow")
            config_text.append("密码:             ", style="cyan")
            config_text.append(f"{masked_pass}\n", style="yellow")
            config_text.append("DeepSeek API Key: ", style="cyan")
            config_text.append(f"{masked_ds_key}\n", style="yellow")
            config_text.append("Gemini API Key:   ", style="cyan")
            config_text.append(f"{masked_gemini_key}\n\n", style="yellow")
            config_text.append("Cookie:           ", style="cyan")
            config_text.append(f"{data.get('Cookie', 'N/A')[:50]}...\n", style="yellow")

            self.console.print(
                Panel(
                    config_text,
                    title="[bold yellow]⚙️ 用户配置信息[/bold yellow]",
                    border_style="yellow",
                    padding=(1, 2),
                )
            )

        except FileNotFoundError:
            self.console.print("[bold red]错误: 未找到 user_data.json 文件！[/bold red]")
        except Exception as e:
            self.console.print(f"[bold red]读取配置文件时出错: {e}[/bold red]")

        input("\n[dim]按 Enter 键返回主菜单...[/dim]")


    def main(self):
        """主程序循环"""
        while True:
            self.display_main_menu()
            choice = Prompt.ask("👉 请输入您的选择 [1-4]")

            if choice == "1":
                self.handle_auto_answer()
            elif choice == "2":
                self.browse_resources_menu()
            elif choice == "3":
                self.display_user_config()
            elif choice == "4":
                self.console.print("\n[bold blue]👋 感谢使用，程序已退出。[/bold blue]\n")
                break
            else:
                self.console.print("\n[bold red]无效输入。[/bold red]")
                time.sleep(1)


if __name__ == "__main__":
    app_ = app_main()
    app_.main()
