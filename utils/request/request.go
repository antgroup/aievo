package request

import (
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/antgroup/aievo/utils/json"
)

func Request(method, url string, param string, resp interface{}, headKvs ...string) error {
	req, err := http.NewRequest(method, url, strings.NewReader(param))
	if err != nil {
		return err
	}
	if len(headKvs)%2 != 0 {
		return errors.New("header be pair")
	}
	if len(headKvs) > 0 {
		for i := 0; i < len(headKvs); i += 2 {
			req.Header.Set(headKvs[i], headKvs[i+1])
		}
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return err
	}
	if resp == nil {
		return nil
	}
	err = json.Unmarshal(body, resp)
	return err
}
