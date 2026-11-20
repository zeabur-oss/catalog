## Introduction

[Umami](https://umami.is/) is a website analytics dashboard. You add a small tracking script to your pages and read visits in the dashboard.

This template runs one Umami instance behind your domain, with its own PostgreSQL database (Service `umami-db-rw`, port `5432`) managed by the CloudNativePG operator.

## Getting started

1. Wait until the service shows as running, then open its domain, for example `https://umami.example.com`.
2. Sign in as `admin`. The password is in Secret `umami-secrets`, key `admin-password`:

   ```sh
   kubectl -n <namespace> get secret umami-secrets -o jsonpath='{.data.admin-password}' | base64 -d
   ```

   The template replaces the published default password at the first start. Change this password in Umami after you sign in.
3. Add your website and put the tracking script on its pages (see Configuration).

The same Secret has the key `app-secret`, which signs sign-in tokens and share links. You do not need it to use Umami.

## Configuration

### Add a website and install the tracking code

Click Websites in the sidebar, then Add website, enter a name and the domain, and save. Click Edit for the website, copy the code under Tracking code and put it in the `<head>` section of your pages. See [Add a website](https://docs.umami.is/docs/add-a-website) and [Collect data](https://docs.umami.is/docs/collect-data).

### Change your password

Click the profile button in the side navigation, then Settings, then Security, then Change password. Before Umami 3.5.0, the button is on the Profile page instead of Security. See [Login](https://docs.umami.is/docs/login).

## Troubleshooting

### Data does not appear

Open the browser developer tools on your site and check the Network tab for requests to your Umami domain. Make sure the script loads without errors. Ad blockers can stop the script from loading. See [Collect data](https://docs.umami.is/docs/collect-data).
