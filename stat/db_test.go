package stat

import (
	"context"
	"database/sql"
	"fmt"
	"math/rand"
	"strings"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/p4gefau1t/trojan-go/common"
)

func TestDBTrafficCounter(t *testing.T) {
	userName := "root"
	password := "password"
	ip := "127.0.0.1"
	port := "3306"
	dbName := "trojan"
	path := strings.Join([]string{userName, ":", password, "@tcp(", ip, ":", port, ")/", dbName, "?charset=utf8"}, "")
	db, err := sql.Open("mysql", path)
	common.Must(err)
	defer db.Close()
	c := &DBTrafficCounter{
		db:          db,
		trafficChan: make(chan *trafficInfo, 1024),
		ctx:         context.Background(),
	}
	simulation := func() {
		for i := 0; i < 100; i++ {
			c.Count("hashhash", rand.Intn(500), rand.Intn(500))
			time.Sleep(time.Duration(int64(time.Millisecond) * rand.Int63n(300)))
		}
		fmt.Println("done")
	}
	for i := 0; i < 100; i++ {
		go simulation()
	}
	go c.dbDaemon()
	time.Sleep(time.Second * 30)
}

func TestDBAuthenticator(t *testing.T) {
	userName := "root"
	password := "password"
	ip := "127.0.0.1"
	port := "3306"
	dbName := "trojan"
	path := strings.Join([]string{userName, ":", password, "@tcp(", ip, ":", port, ")/", dbName, "?charset=utf8"}, "")
	db, err := sql.Open("mysql", path)
	common.Must(err)
	defer db.Close()
	a, err := NewDBAuthenticator(db)
	common.Must(err)
	time.Sleep(time.Second * 5)
	fmt.Println(a.CheckHash("hashhash"), a.CheckHash("jasdlkflfejlqjef"))
}
