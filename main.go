package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"os/exec"
	"reflect"
	"strconv"
	"time"
)

//精灵
type sprite struct {
	row      int
	col      int
	startRow int
	startCol int
}

//Config moji人物图标配置文件 变量首字母大写代表共有参数 外部文件可以调用
type Config struct {
	Player   string `json:"player"`
	Ghost    string `json:"ghost"`
	Wall     string `json:"wall"`  //墙
	Dot      string `json:"dot"`   //点
	Pill     string `json:"pill"`  //药丸
	Death    string `json:"death"` //死亡
	Space    string `json:"space"`
	UseEmoji bool   `json:"use_emoji"`
}

var (
	maze    []string
	player  sprite    //玩家
	ghosts  []*sprite //怪物
	score   int       //积分
	numDots int       //点的数量  这里代表玩家未走到的路
	lives   int       //存活次数
	cfg     Config    //配置

	mazeFile = flag.String("maze-file", "maze.txt", "自定义地图文件地址")
	cfgFile  = flag.String("config-file", "config_emoji.json", "自定义配置文件地址")
)

func typeof(val interface{}) {
	fmt.Println("the val:", val, " type is:", reflect.TypeOf(val))
}

// 清空屏幕
func clearScreen() {
	fmt.Print("\x1b[2J") // 在终端代表清除操作  可以这样操作 在终端输入 `echo "\x1b[2J"`
	moveCursor(0, 0)
}

// 移动光标 到屏幕的row行和col列  以坐标 0,0开始
func moveCursor(row, col int) {
	if cfg.UseEmoji {
		fmt.Printf("\x1b[%d;%df", row+1, col*2+1) // 偏移量 emoji需要2个字符 占2个字节
	} else {
		fmt.Printf("\x1b[%d;%df", row+1, col+1) // 偏移量
	}
}

// 输出背景为蓝色的字符
func withBlueBackground(text string) string {
	return text
	// return "\x1b[44m" + text + "\x1b[0m"
}

func loadCfg(file string) error {
	f, err := os.Open(file)
	if err != nil {
		return err
	}
	defer f.Close()
	decoder := json.NewDecoder(f)
	err = decoder.Decode(&cfg)
	if err != nil {
		return err
	}
	return nil
}

//初始化地图
func loadMaze(file string) error {
	f, err := os.Open(file)
	if err != nil {
		return err
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() { // 逐行读取
		line := scanner.Text() // 将当前行读取的buf转换为string
		maze = append(maze, line)
	}

	for row, line := range maze {
		for col, char := range line {
			switch char {
			// 初始化玩家位置
			case 'P': // 单引号是一个字符,实际上是int32类型的 单引号中不能有多个字符
				player = sprite{row, col, row, col}
			// 初始化怪物
			case 'G':
				ghosts = append(ghosts, &sprite{row, col, row, col}) // 怪物指针集合
			case '.':
				numDots++
			}
		}
	}

	return nil
}

// 输出地图到屏幕
func printScreen() {
	clearScreen() //清除屏幕输出的东西
	for _, line := range maze {
		for _, char := range line {
			switch char {
			case '#':
				// fallthrough // go语言的swithc每个case默认有个break 但如果使用关键字 `fallthrough` 则可以继续执行下面的case
				fmt.Print(withBlueBackground(cfg.Wall))
			case '.':
				fmt.Print(cfg.Dot)
			case 'X':
				fmt.Print(cfg.Pill)
			default:
				fmt.Print(cfg.Space)
			}
		}
		fmt.Println()
	}
	// 打印玩家所在位置
	moveCursor(player.row, player.col)
	fmt.Print(cfg.Player) //输出自己所在位置

	// 打印怪物所在位置
	for _, g := range ghosts {
		moveCursor(g.row, g.col)
		fmt.Print(cfg.Ghost)
	}

	moveCursor(len(maze)+1, 0) //将光标移动到游戏屏幕外远一点

	livesUser := strconv.Itoa(lives) // 转化int为string
	if cfg.UseEmoji {
		livesUser = getLivesAsEmoji()
	}
	// 打印分数
	fmt.Printf("得分: %d \t\t存活次数:%s\n", score, livesUser)
}

func getLivesAsEmoji() string {
	buf := bytes.Buffer{}
	for i := lives; i > 0; i-- {
		buf.WriteString(cfg.Player) // 这里可以使用 `+` 链接两个字符串 但是使用`+`会影响性能 每次for中+链接都会重新分配内存
	}
	return buf.String()
}

func readInput() (string, error) {
	buffer := make([]byte, 100)
	cnt, err := os.Stdin.Read(buffer) //标准输入读取到buffer 最多读取buffer个字节,返回值 读取的字节数
	if err != nil {
		return "", err
	}

	// 这个在屏幕中其实是 字符串 `^[` 传递到程序中则为 `[`  转化为10进制切片为 [27 0 0 0 ...]
	if cnt == 1 && buffer[0] == 0x1b { // `0x1b` 代表16进制的`ESC`按键  十进制的 `27`
		return "ESC", nil
	} else if cnt >= 3 { // 如果输入了 右箭头`->` 这个在屏幕中其实是 `^[[C` 传递到程序中为 `[[C` 转化为10进制切片为 [27 91 67 0 0 ...]
		if buffer[0] == 0x1b && buffer[1] == '[' {
			switch buffer[2] {
			case 'A':
				return "UP", nil
			case 'B':
				return "DOWN", nil
			case 'C':
				return "RIGHT", nil
			case 'D':
				return "LEFT", nil
			}
		}
	}
	return "", nil
}

func initGame() {
	lives = 3 // 三条命
	/**
		终端的运行模式有三种
		1.cooked模式	输入的命令有预处理 如`退格键`,`ctrl+D`,`ctrl+C`,`箭头`等 处理过后再传给程序
		2.cbreak模式	部分预处理 如`ctrl+C`导致程序中断, 但箭头等不会处理
		3.raw模式		不做任何处理 原样返回给程序
		stty (set tty设置终端模式)  `-` 代表不  如  `-echo` 代表不输出屏幕
	**/
	cmd := exec.Command("stty", "cbreak", "-echo")
	cmd.Stdin = os.Stdin

	err := cmd.Run()
	if err != nil {
		log.Fatalln("unable to activate cbreak mode:", err)
	}
}

func closeGame() {
	cmd := exec.Command("stty", "-cbreak", "echo")
	cmd.Stdin = os.Stdin
	err := cmd.Run()
	if err != nil {
		log.Fatalln("unable to activite cooked mode:", err)
	}
}

// 目前player 使用全局变量
func movePlayer(dir string) {
	player.row, player.col = makeMove(player.row, player.col, dir)

	// 声明一个函数 清除 `点`
	removeDot := func(row, col int) {
		maze[row] = maze[row][0:col] + " " + maze[row][col+1:]
	}
	switch maze[player.row][player.col] {
	case '.':
		numDots--
		score++
		removeDot(player.row, player.col)
	case 'X':
		score += 10
		removeDot(player.row, player.col)
	}
}

func makeMove(oldRow, oldCol int, dir string) (newRow, newCol int) {
	newRow, newCol = oldRow, oldCol

	switch dir {
	case "UP":
		newRow--
		if newRow < 0 {
			newRow = len(maze) - 1 // 回到最后一行
		}
	case "DOWN":
		newRow++
		if newRow == len(maze) {
			newRow = 0 // 回到第0行
		}
	case "RIGHT":
		newCol++
		if newCol == len(maze[0]) {
			newCol = 0 //回到第一个列
		}
	case "LEFT":
		newCol--
		if newCol < 0 {
			newCol = len(maze[0]) - 1
		}
	}

	// 遇到墙 则代表不能移动 此次移动失效
	if maze[newRow][newCol] == '#' {
		newRow = oldRow
		newCol = oldCol
	}
	return
}

func moveGhosts() {
	for _, g := range ghosts {
		dir := makeRandDir()
		g.row, g.col = makeMove(g.row, g.col, dir)
	}
}

// 随机生成移动方向
func makeRandDir() string {
	dir := rand.Intn(4)
	move := map[int]string{
		0: "UP",
		1: "DOWN",
		2: "RIGHT",
		3: "LEFT",
	}
	return move[dir]
}

func main() {
	// 命令行控制参数
	flag.Parse()
	// 加载资源
	err := loadMaze(*mazeFile)
	if err != nil {
		log.Println("failed to load maze", err)
		return
	}

	//加配配置文件
	err = loadCfg(*cfgFile)
	if err != nil {
		log.Println("failed to laod cfg", err)
	}

	// 初始化游戏
	initGame()
	defer closeGame()

	// 获取input值 -异步
	input := make(chan string)
	go func(ch chan<- string) {
		for { //循环从标准输入读取
			input, err := readInput() //阻塞
			if err != nil {
				log.Println("failed to read input ", err)
				ch <- "ESC"
			}
			ch <- input
		}
	}(input)

	// 运行游戏
	for {
		// 根据输入移动人物
		select {
		case inp := <-input: //循环读取信道中结束的输入值
			if inp == "ESC" {
				lives = 0
			}
			movePlayer(inp)
		default:
		}

		// 每次移动怪物
		moveGhosts()

		// 更新地图
		printScreen()

		// 检查碰撞
		for _, g := range ghosts {
			if player.row == g.row && player.col == g.col { //玩家和某个怪物相撞
				lives--
				if lives > 0 {
					moveCursor(player.row, player.col)
					fmt.Print(cfg.Death)
					time.Sleep(time.Second)
					player.row, player.col = player.startRow, player.startCol
				}
				break
			}
		}

		// 检测是否结束或关闭
		if numDots == 0 || lives <= 0 {
			if lives <= 0 {
				// 死亡时 输出一个死亡moji图标
				moveCursor(player.row, player.col)
				fmt.Print(cfg.Death)
				moveCursor(len(maze)+2, 0)
			}
			break
		}

		time.Sleep(time.Millisecond * 200) // 定时移动/刷新一次地图
	}
}
