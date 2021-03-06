package main

import (
	"net/http"
	"fmt"
	"os"
	"github.com/shopspring/decimal"
	"net"
	"time"
	"bufio"
	"github.com/patrickmn/go-cache"
	"strconv"
	"regexp"
	"seng468/transaction-server/logger"
)

type QuoteReply struct {
	quote decimal.Decimal
	stock string
	user  string
	time  uint64
	key   string
}

func getReply(msg string) *QuoteReply {
	n1 := re.SubexpNames()
	r2 := re.FindAllStringSubmatch(msg, -1)[0]

	res := map[string]string{}
	for i, n := range r2 {
		res[n1[i]] = n
	}

	quote, _ := decimal.NewFromString(res["quote"])
	timestamp, _ := strconv.ParseUint(res["time"], 10, 64)
	return &QuoteReply{
		quote: quote,
		stock: res["stock"],
		user:  res["user"],
		time:  timestamp,
		key:   res["key"],
	}
}

func quote(user string, stock string, transNum int) (decimal.Decimal, error) {
	quote, found := quoteCache.Get(stock)
	if found {
		d, _ := decimal.NewFromString(quote.(string))
		return d, nil
	}
	conn, err := net.Dial("tcp", "quoteserve.seng:4444")
	if err != nil {
		return decimal.Decimal{}, err
	}
	request := fmt.Sprintf("%s,%s\n", stock, user)
	fmt.Fprintf(conn, request)
	message, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		return decimal.Decimal{}, err
	}
	conn.Close()
	reply := getReply(message)
	go auditServer.QuoteServer("quoteserver", transNum, reply.quote.String(), reply.stock,
		reply.user, reply.time, reply.key)
	quoteCache.Set(reply.stock, reply.quote.String(), cache.DefaultExpiration)
	return reply.quote, nil
}

func quoteHandler(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	user := query.Get("user")
	stock := query.Get("stock")
	transNum, _ := strconv.Atoi(query.Get("transNum"))
	reply, err := quote(user, stock, transNum)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Println("Error receiving quote from quoteserve.seng:4444")
		return
	}
	fmt.Fprintf(w, reply.StringFixed(2))
}

var quoteCache = cache.New(time.Minute, time.Minute)
var re = regexp.MustCompile("(?P<quote>.+),(?P<stock>.+),(?P<user>.+),(?P<time>.+),(?P<key>.+)")
var auditServer = logger.AuditLogger{Addr: "http://" + os.Getenv("auditaddr") + ":" + os.Getenv("auditport")}

func main() {
	http.HandleFunc("/quote", quoteHandler)
	addr := os.Getenv("quoteaddr")
	port := os.Getenv("quoteport")
	fmt.Printf("Quote server listening on %s:%s\n", addr, port)
	if err := http.ListenAndServe(addr + ":" + port, nil); err != nil {
		panic(err)
	}
}

