package http

import (
	"errors"
	log "github.com/shyunku-libraries/go-logger"
	"io"
	"net/http"
	"team.gg-server/util"
)

const (
	EnableDebug = false
)

type GetRequest struct {
	Url           string
	Authorization string
}

type PostRequest struct {
	Url           string
	Body          interface{}
	Authorization string
}

type Response struct {
	Success    bool
	StatusCode int
	Body       []byte
	Stream     io.ReadCloser
	Err        error

	ContentLength int64
}

func Get(req GetRequest) (Response, error) {
	var respBody Response

	if EnableDebug {
		log.Debugf("[HTTP] GET --> %s", req.Url)
	}
	resp, err := getWithRiotPolicy(req.Url)
	if err != nil {
		return respBody, err
	}
	defer resp.Body.Close()

	bodyContent, err := io.ReadAll(resp.Body)
	if err != nil {
		return respBody, err
	}

	respBody.StatusCode = resp.StatusCode
	respBody.Success = resp.StatusCode >= 200 && resp.StatusCode < 300
	respBody.ContentLength = resp.ContentLength
	if respBody.Success {
		respBody.Body = bodyContent
		if EnableDebug {
			log.Debugf("[HTTP] GET <-- %v", respBody.StatusCode)
		}
	} else {
		respBody.Err = errors.New(string(bodyContent))
		if EnableDebug {
			log.Warnf("[HTTP] GET <-X- %s", string(bodyContent))
		}
	}
	return respBody, nil
}

func StreamGet(req GetRequest) (Response, error) {
	var respBody Response

	if EnableDebug {
		log.Debugf("[HTTP] GET --> %s", req.Url)
	}
	resp, err := getWithRiotPolicy(req.Url)
	if err != nil {
		return respBody, err
	}

	respBody.StatusCode = resp.StatusCode
	respBody.Success = resp.StatusCode >= 200 && resp.StatusCode < 300
	respBody.ContentLength = resp.ContentLength
	if respBody.Success {
		respBody.Stream = resp.Body
		if EnableDebug {
			log.Debugf("[HTTP] GET <-- %v", respBody.StatusCode)
		}
	} else {
		bodyContent, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if err != nil {
			return respBody, err
		}
		respBody.Err = errors.New(string(bodyContent))
		if EnableDebug {
			log.Warnf("[HTTP] GET <-X- %s", string(bodyContent))
		}
	}
	return respBody, nil
}

func Post(req PostRequest) (Response, error) {
	var respBody Response

	if EnableDebug {
		log.Debugf("[HTTP] POST --> %s", req.Url)
	}
	requestBody := util.StructToReadable(req.Body)
	request, err := http.NewRequest("POST", req.Url, requestBody)
	if err != nil {
		return respBody, err
	}
	request.Header.Set("Content-Type", "application/json")
	if req.Authorization != "" {
		request.Header.Set("Authorization", "Bearer "+req.Authorization)
	}

	resp, err := http.DefaultClient.Do(request)
	if err != nil {
		return respBody, err
	}
	defer resp.Body.Close()

	bodyContent, err := io.ReadAll(resp.Body)
	if err != nil {
		return respBody, err
	}

	respBody.StatusCode = resp.StatusCode
	respBody.Success = resp.StatusCode >= 200 && resp.StatusCode < 300
	respBody.ContentLength = resp.ContentLength
	if respBody.Success {
		respBody.Body = bodyContent
		if EnableDebug {
			log.Debugf("[HTTP] POST <-- %v", respBody.StatusCode)
		}
	} else {
		respBody.Err = errors.New(string(bodyContent))
		if EnableDebug {
			log.Warnf("[HTTP] POST <-X- %s", string(bodyContent))
		}
	}
	return respBody, nil
}
