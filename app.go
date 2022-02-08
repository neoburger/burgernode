package main

import (
    "bufio"
    "context"
    "encoding/json"
    "fmt"
    "io/ioutil"
    "log"
    "net/http"
    "net/url"
    "os"
    "os/exec"
    "path"
    "strings"
    "sync"

    "github.com/Azure/azure-storage-blob-go/azblob"
)

func main() {
    account, key := os.Getenv("AZURE_STORAGE_ACCOUNT"), os.Getenv("AZURE_STORAGE_ACCESS_KEY")
    cred, err := azblob.NewSharedKeyCredential(account, key)
    if err != nil {
        log.Fatalln(err)
    }

    actions := make(chan func(), 0x1000)
    wg := sync.WaitGroup{}
    defer wg.Wait()
    defer close(actions)
    wg.Add(16)
    for i := 0; i < 16; i++ {
        go func() {
            defer wg.Done()
            for f := range actions {
                f()
            }
        }()
    }

    cmd := exec.Command(os.Getenv("NEONODE"))
    cmd.StdinPipe()
    cmd.Stderr = os.Stderr
    fin, err := cmd.StdoutPipe()
    if err != nil {
        log.Fatalln(err)
    }
    log.Println("debug start")
    if len(os.Getenv("UNTIL")) == 0 {
        log.Println("a")
        resp, err := http.Post("https://n3seed2.ngd.network:10332", "application/json", strings.NewReader(`{"jsonrpc":"2.0","method":"getblockcount","params":[],"id":1}`))
        if err != nil {
            log.Fatalln(err)
        }
        defer resp.Body.Close()
        log.Println("b")
        s, err := ioutil.ReadAll(resp.Body)
        if err != nil {
            log.Fatalln(err)
        }
        log.Println(string(s))
        v := make(map[string]interface{})
        if err := json.Unmarshal([]byte(s), &v); err != nil {
            log.Fatalln(err)
        }
        cmd.Env = append(cmd.Env, fmt.Sprintf("UNTIL=%v", int64(v["result"])))
        log.Println(cmd.Env)
    }
    log.Println(cmd.Env)
    return
    if err := cmd.Start(); err != nil {
        log.Fatalln(err)
    }
    defer cmd.Wait()

    scanner := bufio.NewScanner(fin)
    for scanner.Scan() {
        data := append([]byte{}, scanner.Bytes()...)
        dict := make(map[string]interface{})
        if err := json.Unmarshal(data, &dict); err != nil {
            continue
        }
        actions <- func() {
            if _, err := azblob.UploadBufferToBlockBlob(
                context.Background(),
                data,
                azblob.NewBlockBlobURL(
                    url.URL{
                        Scheme: "https",
                        Host:   fmt.Sprintf("%s.blob.core.windows.net", account),
                        Path:   path.Join("/status", fmt.Sprintf("%v.json", dict["blocknum"])),
                    },
                    azblob.NewPipeline(cred, azblob.PipelineOptions{}),
                ),
                azblob.UploadToBlockBlobOptions{},
            ); err != nil {
                log.Println(err)
            }
        }
    }
}
