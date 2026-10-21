# /financial-bot -> /financial-bot/
location = /financial-bot {
    return 301 /financial-bot/;
}

# Back-end (container financial-bot_backend publicado na porta 8001 do host)
location ^~ /financial-bot/api/ {
    rewrite ^/financial-bot/(.*)$ /$1 break;
    proxy_pass http://172.17.0.1:8001;

    proxy_http_version 1.1;
    proxy_set_header Host              $host;
    proxy_set_header X-Real-IP         $remote_addr;
    proxy_set_header X-Forwarded-For   $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
    proxy_set_header X-Forwarded-Prefix /financial-bot;

    client_max_body_size 20m;
    proxy_read_timeout 120s;
}

# /portal/ -> /financial-bot/portal/
location ^~ /portal/ {
    return 302 /financial-bot$request_uri;
}

# Front-end (build do Vite)
location ^~ /financial-bot/ {
    alias /var/www/html/financial-bot/frontend/dist/;
    index index.html;
    try_files $uri $uri/ /financial-bot/index.html;
}
