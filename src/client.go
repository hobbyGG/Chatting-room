package main

import (
	"fmt"
	"io"
	"net"
	"time"

	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"
)

//go build -ldflags="-H windowsgui"
// func message_box(tell string) {

// }

func main() {

	//连接服务器
	conn, err := net.Dial("tcp", "127.0.0.1:8000")
	conn.Write([]byte("test"))
	if err != nil {
		panic(err)
	}

	var send, chat_room, room *walk.TextEdit //发送框， 聊天框， 列表
	var mw1, mw2 *walk.MainWindow            //登陆窗口、聊天窗口
	var user_n, pass_w *walk.LineEdit        //用户名输入行，密码行
	enter := false                           //登陆是否通过

	//创建登陆窗口
	lw := &MainWindow{
		AssignTo: &mw1,
		Title:    "log in",
		Layout:   VBox{Margins: Margins{Left: 20, Right: 20, Top: 20, Bottom: 20}},
		Size:     Size{Width: 350, Height: 250},
		Children: []Widget{
			HSplitter{
				MaxSize: Size{Width: 100, Height: 25},
				Children: []Widget{
					Label{
						Font:    Font{Family: "微软雅黑", PointSize: 13},
						MinSize: Size{Height: 25},
						Text:    "用户名:",
						Row:     1,
						Column:  1,
					},
					LineEdit{
						Font:     Font{Family: "Consolas", PointSize: 13},
						MaxSize:  Size{Width: 250, Height: 25},
						AssignTo: &user_n,
						Row:      1,
						Column:   1,
					},
				},
			},

			HSplitter{
				MaxSize: Size{Width: 100, Height: 25},
				Children: []Widget{
					Label{
						Font:    Font{Family: "微软雅黑", PointSize: 13},
						MinSize: Size{Height: 25},
						Text:    "密  码:",
					},
					LineEdit{
						Font:         Font{Family: "Consolas", PointSize: 13},
						MaxSize:      Size{Width: 250, Height: 25},
						AssignTo:     &pass_w,
						PasswordMode: true,
					},
				},
			},
			VSplitter{
				Children: []Widget{
					HSeparator{
						MaxSize: Size{Height: 100},
					},
					HSplitter{
						Children: []Widget{
							PushButton{
								MaxSize: Size{Width: 100, Height: 50},
								Text:    "登陆",
								OnClicked: func() {
									account := user_n.Text()
									password := pass_w.Text()
									send_mes := "0 login:" + account + " " + password
									conn.Write([]byte(send_mes))
									buf := make([]byte, 24)
									conn.Read(buf)

									if string(buf[:1]) == "0" {
										walk.MsgBox(mw1, "提醒", "登陆成功!", walk.MsgBoxApplModal)
										enter = true

										go func() { //处理服务器发来的数据
											buf := make([]byte, 1024)
											time.Sleep(time.Second) //等待聊天窗口打开

											for {
												n, err := conn.Read(buf) //获取服务器发来的消息
												if err != nil {
													if err == io.EOF {
														break
													}
													continue
												}

												flag := string(buf[:1])
												switch flag {
												case "0": //指令
													if string(buf[:11]) == "0 get_list:" {
														conn.Write([]byte("0 list"))
														room.SetText("")
														room.AppendText(string(buf[11:n]))
													} else if string(buf[:n]) == "0 /m_err 1" {
														walk.MsgBox(mw2, "注意", "用户不在线或不存在", walk.MsgBoxApplModal)
													} else if string(buf[:n]) == "0 ban" {
														if send.ReadOnly() {
															send.SetText("")
															send.SetReadOnly(false)
														} else {
															send.SetReadOnly(true)
															send.SetText("您已被禁言")
														}
													} else {
														chat_room.AppendText(string(buf[2:n]))
													}
												case "1": //公开聊天消息
													chat_room.AppendText(string(buf[2:n]))
												case "2": //私聊消息
													chat_room.AppendText(string(buf[2:n]))
												}
											}
										}()
										mw1.Close()
									} else {
										walk.MsgBox(mw1, "提醒", "用户名或密码错误!", walk.MsgBoxApplModal)
									}
								},
							},
							PushButton{
								MaxSize: Size{Width: 100, Height: 50},
								Text:    "注册",
								OnClicked: func() {
									account := user_n.Text()
									password := pass_w.Text()
									send_mes := "0 register:" + account + " " + password
									conn.Write([]byte(send_mes))
									buf := make([]byte, 24)
									n, err := conn.Read(buf)
									if err != nil {
										fmt.Println("reg err:", err)
									}
									if string(buf[:n]) == "1" {
										walk.MsgBox(mw1, "提醒", "注册成功!", walk.MsgBoxApplModal)
									} else if string(buf[:n]) == "2" {
										walk.MsgBox(mw1, "提醒", "用户已存在!", walk.MsgBoxApplModal)
									} else if string(buf[:n]) == "3" {
										walk.MsgBox(mw1, "提醒", "用户名最少需要4个字符", walk.MsgBoxApplModal)
									} else {
										walk.MsgBox(mw1, "提醒", "服务出错!", walk.MsgBoxApplModal)
									}
								},
							},
						},
					},
				},
			},
		},
	}
	(*lw).Run()

	if !enter {
		return
	}
	//创建窗口
	cw := &MainWindow{
		AssignTo: &mw2,
		Title:    "Chatting room",
		Layout:   VBox{},
		Size:     Size{Width: 1000, Height: 700},
		Children: []Widget{
			HSplitter{ //分割主窗口,第一次竖直分割
				Children: []Widget{
					VSplitter{ //左半部分
						MinSize: Size{Width: 800},
						MaxSize: Size{Width: 800},
						Children: []Widget{
							VSplitter{ //左半部分再分上下
								MinSize: Size{Height: 500},
								Children: []Widget{
									TextEdit{
										AssignTo:   &chat_room,
										Font:       Font{Family: "微软雅黑", PointSize: 13},
										Background: SolidColorBrush{Color: walk.RGB(0, 200, 255)},
										ReadOnly:   true,
										VScroll:    true,
									},
								},
							},
							TextEdit{
								Font:     Font{Family: "微软雅黑", PointSize: 13},
								AssignTo: &send,
							},
						},
					},
					VSplitter{ //右半部分，上是用户列表，下是按钮
						Children: []Widget{
							Label{
								MaxSize: Size{Width: 10, Height: 10},
								MinSize: Size{Width: 10, Height: 10},
								Text:    "user list",
							},
							TextEdit{
								Font:     Font{Family: "consolas", PointSize: 13},
								AssignTo: &room,
								VScroll:  true,
							},
							PushButton{ //按钮，发消息
								Text: "发送",
								OnClicked: func() { //按下按钮后的操作
									//获取文本框的数据
									buf := []byte(send.Text())
									send.SetText("")
									//向服务器发送数据
									_, err := conn.Write(buf)
									if err != nil {
										fmt.Println("write err:", err)
										return
									}
								},
							},
							PushButton{
								Text:       "退出",
								Background: SolidColorBrush{Color: walk.RGB(120, 0, 0)},
								OnClicked: func() {
									conn.Write([]byte("/exit"))
									time.Sleep(time.Second / 2)
									mw2.Close()
								},
							},
						},
					},
				},
			},
		},
	}
	(*cw).Run()
}

