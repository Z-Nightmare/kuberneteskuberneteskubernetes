## File Tree

```file
.
├── LICENSE
├── Makefile
├── README.md
├── config
│   └── config-example.yaml
├── deploy
│   ├── docker
│   │   └── Dockerfile
│   └── kubernetes
│       └── app.yaml
├── docker-compose.yaml
└── main.go
```

## Init

Fork and then rename the project.

### Replace variable as you want

variable | example
---|---
$org|github.com/zeusro
$project-name|go-example
$app|go-example

### Init project

    MODULE="github.com/zeusro/go-template" make init
    make fix-dep
    
