package main

import (
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"
)

var online = make(map[string]*user_type)  //记录当前在线用户
var user_hashmap = make([]string, 1024)   //用于给用户排序
var open_ch = make(chan string)           //公用管道，通过该管道转发
var open_ch_flag = make(chan bool, 1)     //公用管道信号量
var fp *os.File                           //↓文件指针
var info_string string                    //↓文件内容
const src_f = "./information/account.txt" //账号信息路径
var state_frame, user_frame, user_db *walk.TextEdit

type user_type struct {
	u_ch chan string // user message channel
	name string
	add  string
	num  int
	flag bool
	conn net.Conn
}

// 通过公用管道向用户自己的管道传输数据
func handle_open_ch() {
	for {
		mes := <-open_ch
		<-open_ch_flag
		for _, user := range online {
			user.u_ch <- mes
		}
		open_ch_flag <- true
	}
}
// 处理用户登陆
func handle_login(conn net.Conn, num int) {

	buf := make([]byte, 1024*4)
	var user_name string

	for {
		n, err := conn.Read(buf) //从客户端接受数据
		if err != nil {
			if err == io.EOF {
				continue
			}
			fmt.Println(err)
			return
		}

		if string(buf[:8]) == "0 login:" { //如果是登陆请求
			cmp := string(buf[8:n])
			//先检查用户名长度是否大于4
			if len(get_user_name(cmp)) < 4 {
				conn.Write([]byte("2"))
			} else {
				if _, ps := info_validate(cmp); ps { //ps是密码，密码通过则代表通过
					conn.Write([]byte("0"))        //验证成功
					user_name = get_user_name(cmp) //上传用户名
					break
				} else {
					conn.Write([]byte("2"))
				}
			}

		} else if string(buf[:11]) == "0 register:" {
			new_acunt := string(buf[11:n]) + "\n"
			cmp_s := get_user_name(new_acunt) //用户名字符串

			if len(cmp_s) < 4 {
				conn.Write([]byte("3"))

			} else {

				if un, _ := info_validate(cmp_s); un { //检查是否与已有用户重复,un为true则有重复
					conn.Write([]byte("2"))

				} else { //没有重复则创建新的用户信息
					fp.Write([]byte(new_acunt)) //写入新用户信息
					err = fp.Close()            //及时保存文件
					if err != nil {
						fmt.Println("close err:", err)
					}
					info_string += new_acunt //直接更新文件信息字符串，不用重新打开文件读取
					fp, err = os.OpenFile(src_f, os.O_RDWR|os.O_APPEND, 0666)
					if err != nil {
						fmt.Println(err)
					}
					conn.Write([]byte("1")) //注册成功
					fp, err = os.OpenFile(src_f, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0666)
				}
			}

		}
	}
	handle_user(conn, user_name, num)
}
// 处理用户服务
func handle_user(conn net.Conn, user_name string, num int) {
	//创建该用户并将其加入管理当前在线用户的map中
	quit := make(chan bool)
	has_data := make(chan bool)
	add := conn.RemoteAddr().String()
	user := user_type{make(chan string), user_name, add, num, false, conn}
	online[add] = &user

	defer func() {
		delete(online, user.add)
		user_hashmap[user.num] = ""
		online_string := "0 get_list:"
		for _, name := range user_hashmap {
			if name == "" {
				continue
			}
			online_string += (name + "\r\n")
		}
		chan_write(open_ch, online_string)
		t := make([]byte, 10)
		conn.Read(t)
		conn.Close()
		user_frame.SetText(online_string[11:])
	}()

	//新开一个协程专门处理该用户消息
	go func() {
		//新开一个协程专门接收用户发出的消息
		for mes := range user.u_ch {
			conn.Write([]byte(mes))
		}
	}()
	go func() {
		buf := make([]byte, 1024*4) //用于接收消息的切片

		for {
			n, err := conn.Read(buf)
			if err != nil {
				quit <- true
				if err == io.EOF {
					break
				}
				fmt.Println("read err:", err)
				break
			}

			buf_string := string(buf[:n])
			if string(buf_string) == "0 list" {
			} else if buf_string == "/exit" {
				quit <- true
				return
			} else if string(buf[:3]) == "/m:" { // /m: username say
				var exist bool
				mes_buf := strings.Split(buf_string, " ")
				for _, t_user := range online {
					if t_user.name == mes_buf[1] {
						exist = true
						t_user.u_ch <- ("2 " + "[" + get_time() + "]" + user.name + " say to you : " + mes_buf[2] + "\r\n")
						break
					}
				}
				if !exist { //该用户不存在或不在线
					conn.Write([]byte("0 /m_err 1"))
				}
			} else {
				has_data <- true
				chan_write(open_ch, "1 "+"["+get_time()+"]"+user.name+" : "+buf_string+"\r\n")
			}
			buf_string = ""
		}
	}()

	//新建完用户后刷新一下用户列表
	func() {
		online_string := "0 get_list:"

		//为当前用户排序，存入user_hashmap
		for _, user := range online {
			if user.flag { //已经装入hashmap
				continue
			} else if user_hashmap[user.num] == "" { //检查哈希碰撞
				user_hashmap[user.num] = user.name
				user.flag = true
			} else {
				user.num++
			}
		}
		for _, name := range user_hashmap {
			if name == "" {
				continue
			}
			online_string += (name + "\r\n")
		}
		// conn.Write([]byte(online_string))
		chan_write(open_ch, online_string)
		user_frame.SetText(online_string[11:])
	}()
	fmt.Println(user.name + " log in")
	state_frame.AppendText(user.name + " log in" + "\r\n")
	for {
		select {
		case <-quit:
			return
		case <-has_data:

		case <-time.After(time.Hour * 2):
			return
		}
	}
}
// 获取账号信息中的用户名
func get_user_name(s string) (user_name string) {
	i := 0
	user_name = ""
	for {
		if s[i] == ' ' {
			break
		}
		user_name += string(s[i])
		i++
	}
	return
}
// 验证用户登陆信息是否与保存的文件一致,传入用户名和密码
func info_validate(s string) (un, ps bool) {
	//对信息进行分组并比较，有完全相同则返回true
	aim := strings.Split(info_string, "\n")
	for _, value := range aim {
		if value == "" {
			continue
		}
		if get_user_name(value) == s { //有用户名相同
			un = true
			ps = false
		}
		if value == s { //用户名和密码一致
			un = true
			ps = true
			return
		}
	}
	return
}
// 取得当前时间
func get_time() (now string) { //2023-02-03 14:30:38.726161 +0800 CST m=+1.493701301
	t1, t2, _ := time.Now().Clock()
	h := strconv.Itoa(t1)
	m := strconv.Itoa(t2)

	if len(h) == 1 {
		h = "0" + h
	}
	if len(m) == 1 {
		m = "0" + m
	}

	now = h + ":" + m
	return
}
func chan_write(open_ch chan string, s string) {
	<-open_ch_flag
	open_ch <- s
	open_ch_flag <- true
}
func sconv(info_string string) string {
	info_text := ""
	ts := strings.Split(info_string, "\n")
	for _, t := range ts {
		n := (strings.Split(t, " "))[0]
		info_text += (n + "\r\n")
	}
	return info_text
}
// 服务端
func main() {
	var err error
	fp, err = os.OpenFile(src_f, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0666)
	if err != nil {
		fmt.Println("main err:")
		panic(err)
	}
	open_ch_flag <- true
	defer fp.Close() //服务端关闭后保存文件

	//获取账号信息
	account_info := make([]byte, 1024*4)
	for { //获取所有账号信息
		n, err := fp.Read(account_info)
		if err != nil {
			if err == io.EOF {
				break
			}
		}
		info_string += string(account_info[:n])
	}

	//监听端口
	listenner, err := net.Listen("tcp", "127.0.0.1:8000")
	if err != nil {
		fmt.Println("listen err:", err)
	} else {
		fmt.Println("boot successfully!")
	}

	//启动专门处理公用管道——消息转发
	go handle_open_ch()

	go func() {
		n := 0
		for {
			//阻塞等待用户接入
			conn, err := listenner.Accept()
			test_buf := make([]byte, 24)
			conn.Read(test_buf) //清空conn里的初始数据
			if err != nil {
				fmt.Println("listen err:", err)
				continue
			}
			go handle_login(conn, n) //处理用户登陆
			n++                      //每处理一个用户，下一个用户的代号+1，用于排序
			n %= 1024                //最多1024名用户
		}
	}()

	var m_wp, sub_wp1, sub_wp2 *walk.MainWindow
	var t1, t2 *walk.LineEdit
	//禁言框
	subw1 := MainWindow{
		Title:    "禁言用户",
		AssignTo: &sub_wp1,
		Size:     Size{Width: 300, Height: 120},
		Layout:   VBox{},
		Children: []Widget{
			HSplitter{
				Children: []Widget{
					Label{
						Text: "用户名",
						Font: Font{Family: "微软雅黑", PointSize: 13},
					},
					LineEdit{
						AssignTo: &t1,
						Font:     Font{Family: "微软雅黑", PointSize: 13},
					},
				},
			},
			PushButton{
				Text: "确定",
				OnClicked: func() {
					user_name := t1.Text()
					if len(user_name) < 4 {
						walk.MsgBox(sub_wp1, "提示", "用户名最少4位", walk.MsgBoxApplModal)
					} else {
						find := false
						for _, user := range online {
							if user.name == user_name {
								user.conn.Write([]byte("0 ban"))
								find = true
							}
						}
						if !find {
							walk.MsgBox(sub_wp1, "提示", "未找到该用户", walk.MsgBoxApplModal)
						} else {
							sub_wp1.Close()
						}
					}
				},
			},
		},
	}
	//删除框
	subw2 := MainWindow{
		Title:    "删除用户",
		AssignTo: &sub_wp2,
		Size:     Size{Width: 300, Height: 500},
		Layout:   VBox{},
		Children: []Widget{
			Label{
				Text: "用户名",
				Font: Font{Family: "微软雅黑", PointSize: 13},
			},
			//显示所有用户名
			TextEdit{
				AssignTo:   &user_db,
				ReadOnly:   true,
				MaxSize:    Size{Width: 300},
				VScroll:    true,
				Font:       Font{Family: "微软雅黑", PointSize: 13},
				Background: SolidColorBrush{Color: walk.RGB(0, 150, 150)},
				//  func() string {
				// 	info_text := ""
				// 	ts := strings.Split(info_string, "\n")
				// 	for _, t := range ts {
				// 		n := (strings.Split(t, " "))[0]
				// 		info_text += (n + "\r\n")
				// 	}
				// 	return info_text
				// }(),
			},

			HSplitter{
				Children: []Widget{
					Label{
						Text: "用户名",
						Font: Font{Family: "微软雅黑", PointSize: 13},
					},
					LineEdit{
						AssignTo: &t2,
						Font:     Font{Family: "微软雅黑", PointSize: 13},
					},
				},
			},
			PushButton{
				Text: "刷新用户列表",
				OnClicked: func() {
					user_db.SetText(sconv(info_string))
				},
			},
			PushButton{
				Text: "确定",
				OnClicked: func() {
					user_name := t2.Text()
					if len(user_name) < 4 {
						walk.MsgBox(sub_wp2, "提示", "用户名最少4位", walk.MsgBoxApplModal)
					} else {
						find := false
						rep_obj := ""
						ts := strings.Split(info_string, "\n")
						for _, t := range ts {
							u := (strings.Split(t, " "))[0]
							if u == user_name {
								rep_obj = t
								find = true
							}
						}
						if !find {
							walk.MsgBox(sub_wp2, "提示", "不存在该用户", walk.MsgBoxApplModal)
						} else {
							info_string = strings.Replace(info_string, (rep_obj + "\n"), "", 1)
							fp.Close()
							fp, err = os.OpenFile(src_f, os.O_WRONLY|os.O_TRUNC, 0666)
							if err != nil {
								walk.MsgBox(sub_wp2, "提示", "打开文件错误", walk.MsgBoxApplModal)
							}
							fp.WriteString(info_string)
							fp.Close()
							fp, err = os.OpenFile(src_f, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0666)
							user_db.SetText(sconv(info_string))
							sub_wp2.Close()
						}

					}
				},
			},
		},
	}
	mw := &MainWindow{
		AssignTo: &m_wp,
		Title:    "Administrator",
		Layout:   VBox{Margins: Margins{Left: 20, Right: 20, Top: 20, Bottom: 20}},
		Size:     Size{Width: 800, Height: 600},
		Children: []Widget{
			HSplitter{
				Children: []Widget{
					VSplitter{
						Children: []Widget{
							Label{
								Text: "当前状态",
								Font: Font{Family: "微软雅黑", PointSize: 13},
							},
							TextEdit{
								AssignTo:   &state_frame,
								Font:       Font{Family: "微软雅黑", PointSize: 13},
								Background: SolidColorBrush{Color: walk.RGB(255, 255, 255)},
								ReadOnly:   true,
								VScroll:    true,
							},
							PushButton{
								Text: "禁言用户",
								Font: Font{Family: "微软雅黑", PointSize: 13},
								OnClicked: func() {
									subw1.Run()
								},
							},
						},
					},
					VSplitter{
						Children: []Widget{
							Label{
								Text: "用户",
								Font: Font{Family: "微软雅黑", PointSize: 13},
							},
							TextEdit{
								AssignTo:   &user_frame,
								Font:       Font{Family: "微软雅黑", PointSize: 13},
								Background: SolidColorBrush{Color: walk.RGB(255, 255, 255)},
								ReadOnly:   true,
								VScroll:    true,
							},
							PushButton{
								Text: "删除用户",
								Font: Font{Family: "微软雅黑", PointSize: 13},
								OnClicked: func() {
									subw2.Run()
								},
							},
						},
					},
				},
			},
		},
	}
	mw.Run()
}
