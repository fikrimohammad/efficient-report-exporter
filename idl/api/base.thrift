namespace go api

struct BaseResponse {
    1: string    code    (api.body="code")
    2: string    message (api.body="message")
}
