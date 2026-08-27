# SSL Certificates for ALB

This directory should contain SSL certificates for the Application Load Balancer.

## Option 1: Use Alibaba Cloud SSL Certificate (Recommended)

For production, use the Alibaba Cloud SSL Certificate Service:

1. Upload or purchase a certificate in the Alibaba Cloud console
2. Get the certificate ID
3. Set in your terraform.tfvars:
   ```hcl
   ssl_certificate_id = "your-certificate-id"
   ```

## Option 2: Local Certificate Files (Development/Testing)

For development or testing, place your certificates here:

```
terraform/certs/
├── server.crt    # SSL certificate (PEM format)
└── server.key    # Private key (PEM format)
```

### Generate Self-Signed Certificate (Development Only)

```bash
# Generate a self-signed certificate valid for 365 days
openssl req -x509 -nodes -days 365 -newkey rsa:2048 \
  -keyout server.key \
  -out server.crt \
  -subj "/CN=*.accountant-crm.com/O=Accountant CRM/C=GB"
```

**WARNING:** Self-signed certificates should NEVER be used in production.

## Files in This Directory

- `README.md` - This file
- `server.crt` - SSL certificate (not committed to git)
- `server.key` - Private key (not committed to git)

## Security Notes

- **NEVER commit certificate private keys to version control**
- Use `.gitignore` to exclude `*.key` and `*.crt` files
- For production, always use properly issued SSL certificates
- Rotate certificates before expiry
