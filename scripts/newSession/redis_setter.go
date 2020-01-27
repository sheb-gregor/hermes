package main

import (
	"encoding/base32"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/lancer-kit/noble"
	"gitlab.inn4science.com/ctp/hermes/config"
	"gitlab.inn4science.com/ctp/hermes/sessions"
)

func main() {
	cfg := config.RedisConf{
		DevMode:       false,
		MaxIdleConn:   5,
		MaxActiveConn: 0,
		IdleTimeout:   0,
		PingInterval:  0,
		Password:      noble.Secret{}.New("raw:"),
		Host:          "redis:6379",
	}

	storage, err := sessions.NewStorage(cfg)
	if err != nil {
		log.Fatal(err)
	}

	for i := 0; i < 10; i++ {
		binary, _ := uuid.New().MarshalBinary()
		token := base32.StdEncoding.EncodeToString(binary)

		time.Sleep(time.Second)
		uid := uuid.New().String()

		println("TOKEN:", token)
		println("UUID:", uid)

		err = storage.SaveAsJSON(token, &sessions.Session{
			Token:          token,
			UserID:         uid,
			ExpirationTime: int64(time.Hour),
			Active:         true,
		}, int64(time.Hour))
	}
}

var _ = `
TOKEN: PQD2ATOUL5E53CI5AWEODV7CQE======
UUID: b1b9832c-d519-410d-8881-7a28324ddc75

TOKEN: NUTCLY4BCVCUJK5X4SEEN7BTTQ======
UUID: 9f7c3c11-4cb5-4da1-8b68-81ab21221298

TOKEN: NUSF666EAJBCTHPBN7YDZ6C5IE======
UUID: f91f0531-8609-47d3-a464-f231b99e5a70

TOKEN: B64VL35XSFAONLX3V64NOGLR7E======
UUID: 9ec6a529-d20a-47e5-865b-71e3047054d6

TOKEN: WWAK66RKANBXBA3FEWCBDO6FIU======
UUID: 2639acf6-a88f-44da-bd34-562b2d08bd81

TOKEN: SXUOHSRRZZH7VBYD66LNSWPZVI======
UUID: 493e93f0-611c-4a30-980e-bec1ac5e3fa3

TOKEN: DJOFX6MZCBH7FAHRYKDV7OGG3E======
UUID: 08380f2b-2eb1-45b3-9e4a-e8300a9cb873

TOKEN: 7AKGTU7FCRE53LGBJTCYP5J7AI======
UUID: ec73ce00-7a57-4c48-824e-1de2dd1d174d

TOKEN: LJZ4O6CCC5BR3NNOMC7Z55GAW4======
UUID: 4429a8ef-2951-4525-8654-54dfcf92e1d8

TOKEN: W7F64HTNMZE7JPR6YHLT67CWVA======
UUID:  070ba57a-07cb-4687-9fa2-0f427ec88525



TOKEN: 6UFOS4OR2RE25HQ2S57BAACXTU======
UUID: c5eafb7e-e5dc-43ed-8109-83e1d7b12a18
TOKEN: RTC3PEGVJFCE5IHV3SQXIO5QCU======
UUID: 5d5d1e74-fbcd-4570-8a9f-752874717fbe
TOKEN: NABXYQENQZDXTPMSNCMA5GMKUM======
UUID: 722c40a4-ecf4-4c31-8123-35f60e48fde2
TOKEN: RGTLMP4LHFB55G2LHVSOW4MWX4======
UUID: ff401dc0-52d5-43f9-b0a7-46fa88e4c797
TOKEN: XRUGJKXPVJD7DPWXE362FUXCVU======
UUID: 96ecdb74-bd3f-4bd7-b005-9a5ecc801a2a
TOKEN: Y4VLLNK3SNBRNFPUI6W2VDNLFI======
UUID: db5c18e9-f567-4db5-b8d2-b55f474e0a99
TOKEN: QRF6JSSW3BBGRPSJBRXC3YXC7U======
UUID: 280cee8d-398a-4dac-aee7-82caa00e491d
TOKEN: FBFAXSZQ6JELRBRCKXFOVZHYAU======
UUID: d2e7b132-79db-4d02-8501-3462c6370c0a
TOKEN: UQECCQCQ5BCLVDNU7UTG6Y3C3U======
UUID: 016cbe50-8200-424a-a5fe-a52f0df90057
TOKEN: NCXYFHJB3NCOJIOSWP53VMUFRA======
UUID: 059a292d-4937-4f36-b02d-62e218e9a879

`
