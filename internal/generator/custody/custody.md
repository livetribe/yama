# Set Aside Action

| scenario | file               | exists | backup<br>exists | action                                                   | notes                                                                                                                                            |
|:--------:|--------------------|:------:|:----------------:|----------------------------------------------------------|--------------------------------------------------------------------------------------------------------------------------------------------------|
|    1     | `lifecycle_gen.go` |   no   |        no        | do nothing                                               |                                                                                                                                                  |
|    2     | `lifecycle_gen.go` |   no   |       yes        | do nothing                                               | Not likely to happen, but possibly `yama` died in the middle of execution.                                                                       |
|    3     | `lifecycle_gen.go` |  yes   |        no        | move to `lifecycle_gen.go.bak`                         |                                                                                                                                                  |
|    4     | `lifecycle_gen.go` |  yes   |       yes        | keep `lifecycle_gen.go.bak`, delete `lifecycle_gen.go` | Normally, I think this would be a panic situation, but I think that since this is so unlikely, we just go ahead and set aside the file we found. |
|    5     | `wire_gen.go`      |   no   |        no        | do nothing                                               |                                                                                                                                                  |
|    6     | `wire_gen.go`      |   no   |       yes        | do nothing                                               | Not likely to happen, but possibly `wire` died in the middle of execution.                                                                       |
|    7     | `wire_gen.go`      |  yes   |        no        | move to `wire_gen.go.bak`                              |                                                                                                                                                  |
|    8     | `wire_gen.go`      |  yes   |       yes        | keep `wire_gen.go.bak`, delete `wire_gen.go`           | Normally, I think this would be a panic situation, but I think that since this is so unlikely, we just go ahead and set aside the file we found. |

# Cleanup Action

| scenario | file               | exists | backup<br>exists | action                            | notes                                                            |
|:--------:|--------------------|:------:|:----------------:|-----------------------------------|------------------------------------------------------------------|
|    9     | `lifecycle_gen.go` |   no   |        no        | do nothing                        | Either `wire`/`yama` died or there are no stubs in this package. |
|    10    | `lifecycle_gen.go` |   no   |       yes        | restore `lifecycle_gen.go.bak`  | Likely `wire`/`yama` died or there are no stubs in this package. |
|    11    | `lifecycle_gen.go` |  yes   |        no        | do nothing                        |                                                                  |
|    12    | `lifecycle_gen.go` |  yes   |       yes        | delete `lifecycle_gen.go.bak`   |                                                                  |
|    13    | `wire_gen.go`      |   no   |        no        | do nothing                        |                                                                  |
|    14    | `wire_gen.go`      |   no   |       yes        | restore `wire_gen.go.bak`       | Likely `wire` died or there are no stubs in this package.        |
|    15    | `wire_gen.go`      |  yes   |        no        | delete `wire_gen.go`              | Only if there was no error backing up an original `wire_gen.go`. |
|    16    | `wire_gen.go`      |  yes   |       yes        | replace `wire_gen.go` with backup |                                                                  |
