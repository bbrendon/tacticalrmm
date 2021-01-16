package api

import (
	"bufio"
	"errors"
	"fmt"
	"log"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
	nats "github.com/nats-io/nats.go"
	"github.com/ugorji/go/codec"
	rmm "github.com/wh1te909/rmmagent/shared"
)

var rClient = resty.New()

func getAPI(apihost, natshost string) (string, string, error) {
	if apihost != "" && natshost != "" {
		return apihost, natshost, nil
	}

	f, err := os.Open(`/etc/nginx/sites-available/rmm.conf`)
	if err != nil {
		return "", "", err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if strings.Contains(scanner.Text(), "server_name") && !strings.Contains(scanner.Text(), "301") {
			r := strings.NewReplacer("server_name", "", ";", "")
			s := strings.ReplaceAll(r.Replace(scanner.Text()), " ", "")
			return fmt.Sprintf("https://%s/natsapi", s), fmt.Sprintf("tls://%s:4222", s), nil
		}
	}
	return "", "", errors.New("unable to parse api from nginx conf")
}

func Listen(apihost, natshost string, debug bool) {
	api, natsurl, err := getAPI(apihost, natshost)
	if err != nil {
		log.Fatalln(err)
	}

	if debug {
		fmt.Println("Api base url: ", api)
		fmt.Println("Nats connection url: ", natsurl)
	}

	rClient.SetHostURL(api)
	rClient.SetTimeout(30 * time.Second)
	natsinfo, err := rClient.R().SetResult(&NatsInfo{}).Get("/natsinfo/")
	if err != nil {
		log.Fatalln(err)
	}

	opts := []nats.Option{
		nats.Name("TacticalRMM"),
		nats.UserInfo(natsinfo.Result().(*NatsInfo).User,
			natsinfo.Result().(*NatsInfo).Password),
		nats.ReconnectWait(time.Second * 5),
		nats.RetryOnFailedConnect(true),
		nats.MaxReconnects(-1),
		nats.ReconnectBufSize(-1),
	}

	nc, err := nats.Connect(natsurl, opts...)
	if err != nil {
		log.Fatalln(err)
	}

	nc.Subscribe("*", func(msg *nats.Msg) {
		var mh codec.MsgpackHandle
		mh.RawToString = true
		dec := codec.NewDecoderBytes(msg.Data, &mh)

		switch msg.Reply {
		case "hello":
			go func() {
				var p *rmm.CheckIn
				if err := dec.Decode(&p); err == nil {
					rClient.R().SetBody(p).Patch("/checkin/")
				}
			}()
		case "osinfo":
			go func() {
				var p *rmm.CheckInOS
				if err := dec.Decode(&p); err == nil {
					rClient.R().SetBody(p).Put("/checkin/")
				}
			}()
		case "winservices":
			go func() {
				var p *rmm.CheckInWinServices
				if err := dec.Decode(&p); err == nil {
					rClient.R().SetBody(p).Put("/checkin/")
				}
			}()
		case "publicip":
			go func() {
				var p *rmm.CheckInPublicIP
				if err := dec.Decode(&p); err == nil {
					rClient.R().SetBody(p).Put("/checkin/")
				}
			}()
		case "disks":
			go func() {
				var p *rmm.CheckInDisk
				if err := dec.Decode(&p); err == nil {
					rClient.R().SetBody(p).Put("/checkin/")
				}
			}()
		case "loggedonuser":
			go func() {
				var p *rmm.CheckInLoggedUser
				if err := dec.Decode(&p); err == nil {
					rClient.R().SetBody(p).Put("/checkin/")
				}
			}()
		case "software":
			go func() {
				var p *rmm.CheckInSW
				if err := dec.Decode(&p); err == nil {
					rClient.R().SetBody(p).Put("/checkin/")
				}
			}()
		case "syncmesh":
			go func() {
				var p *rmm.MeshNodeID
				if err := dec.Decode(&p); err == nil {
					rClient.R().SetBody(p).Post("/syncmesh/")
				}
			}()
		}
	})

	nc.Flush()

	if err := nc.LastError(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	runtime.Goexit()
}
