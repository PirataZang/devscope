# /peppercore-admin -> /peppercore-admin/
location = /peppercore-admin {
    return 301 /peppercore-admin/;
}

# Front-end (build do Vite)
location ^~ /peppercore-admin/ {
    alias /var/www/html/peppercore-admin/dist/;
    index index.html;
    try_files $uri $uri/ /peppercore-admin/index.html;
}
