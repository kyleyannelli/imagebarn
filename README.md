# ImageBarn

ImageBarn offers a distinctive way to share images using your Gmail (Google account) via OAuth2. An admin, approves users before they can participate. Once approved, users can upload images, default max of 5, which become available at random through an authenticated API endpoint.

When an image is accessed via the API, it magically fades away from the uploader's account, signifying its one-time use. This feature provides an engaging method for sharing photos unpredictably. Users can also witness their photos seamlessly transitioning into whatever content you've configured the endpoint to display.

**Potential Use Cases:**

- **Event Photo Sharing:** Perfect for occasions where photos can be shared in a random order, adding an element of surprise.
- **Collaborative Projects:** Great for group contributions where each image adds an unpredictable element to a shared creative project, such as a randomized photo gallery.
- **Ephemeral Content:** Great for creating time-limited content that enhances exclusivity and encourages immediate engagement.
- **Interactive Experiences:** Useful for games or activities that involve unveiling images unpredictably.

ImageBarn transforms the way images are shared, combining simplicity with a touch of magic to create an elegant experience.

# Getting Started

### Installer Script (Recommended - Debian & Ubuntu Only)
`sudo bash -c "$(curl -fsSL https://raw.githubusercontent.com/kyleyannelli/imagebarn/v1.0.0/install.sh)"`

Use the service installer script. The script moves the binary to a suitable location, creates an imagebarn user, then creates a systemd service for ImageBarn.
It is highly recommended to use NGINX or Caddy to use HTTPS. ImageBarn uses GoFiber, which is built on fasthttp. fasthttp does not support http/2, so https is left to a reverse proxy. This is why ImageBarn does not log path access.

### Manual Setup
If you don't use the service installer script: Vips is required to process heic files, mainly from iPhones, to webp. There is not a single web browser that supports heic natively. Install on most distributions with `sudo apt install libvips-dev` or `brew install vips`.

### Setting up your environment (Skip if you used the installer script)
The .env.example has everything you need to get started:

```.env
GOOGLE_CLIENT_ID="ABC123.app"
GOOGLE_CLIENT_SECRET="123CBD--L"
BEARER_TOKEN="PLEASE_GENERATE_A_SECURE_TOKEN"
# Make this the base uri. If you want your ImageBarn at yoursite.com just put https://yoursite.com. This depends on how you setup your DNS.
BASE_URI="https://imagebarn.mysite.com"
ADMIN_USER="kyleyannelli@gmail.com"
# How many concurrent images can be processed.
IMAGE_WORKERS=1
# Enter your trusted proxies in a comma separated list
# Local host is already covered with 127.0.0.1 & ::1
# Ex 1: "192.168.1.56"
# Ex 2: "10.0.0.66, 10.0.0.40, 10.0.0.34"
# Ex 3 (using nginx locally or not using a proxy): ""
TRUSTED_PROXIES="10.0.0.66, 10.0.0.34, 10.0.0.40"
UPLOAD_LIMIT_MB="35"
```

You will need to setup the Google OAuth Consent Screen, as well as Google client id & secret from [here](https://support.google.com/cloud/answer/6158849?hl=en).
- The only scope you need is `/auth/userinfo.email`.
- You will need to define the callback route as `https://your.site.com/auth/callback`

Once you have completed those steps, place the `BASE_URI` in the .env. So if you are hosting the index of the site as `https://your.site.com/`, that is what you will enter for `BASE_URI`.

Then, place your google client ID & secret in the appropriate spots of your .env

If you are running a proxy, cloudflare tunnel, nginx, caddy, and the like. You will need to enter their IPs into `TRUSTED_PROXIES`. Refer to the comment in the .env for how to enter multiple IPs (IPv6 supported).

Next, enter your email address for `ADMIN_USER`. There is only ONE admin user. The ONE admin user is the ONLY user who can approve or disapprove users which have signed in. Attempting to approve or disapprove of the admin user will result in a 400 bad request error.
- An approved user can upload and see the images the currently have uploaded. They can technically delete an image via the API, but there is no interaction for the approved user to do this on the webpage.
- A disapproved user cannot do anything. They will be greeted with the "awaiting approval" screen. The app never requires a page refresh apart from the Google OAuth2 sign-in.

Finally, the `BEARER_TOKEN`. I ask you generate a 32 character string and place it in the .env. This will be the authentication used for the only route GET `/api/image`.

# Acknowledgements
ImageBarn was developed with the help of the following open-source tools:
- [Fiber](https://github.com/gofiber/fiber) a lightweight server framework that made backend development straightforward.
- [HTMX](https://github.com/bigskysoftware/htmx) enabled a dynamic frontend with minimal JavaScript.
- [Pico](https://github.com/picocss/pico) provided a simple and pretty styling solution for the webpages.
