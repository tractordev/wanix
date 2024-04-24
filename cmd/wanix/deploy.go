package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"path"
	"strings"
	"text/template"
	"time"

	"github.com/auth0/go-auth0"
	"github.com/auth0/go-auth0/management"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/huh/spinner"
	"github.com/charmbracelet/lipgloss"
	"github.com/cli/oauth/api"
	"github.com/cli/oauth/device"
	"github.com/google/go-github/v58/github"
	"github.com/olekukonko/tablewriter"
	"tractor.dev/toolkit-go/engine/cli"
	"tractor.dev/wanix/boot"
	"tractor.dev/wanix/internal/auth0flow"
)

func deployCmd() *cli.Command {
	var enableAuth bool
	var requireAuth bool
	var subpath string

	cmd := &cli.Command{
		Usage: "deploy [domain]",
		Short: "deploy WANIX static site to GitHub Pages",
		Args:  cli.MaxArgs(1),
		Run: func(ctx *cli.Context, args []string) {
			domain := ""
			if len(args) > 0 {
				domain = args[0]
			}
			if requireAuth {
				enableAuth = true
			}
			deploy(domain, subpath, enableAuth, requireAuth)
		},
	}
	cmd.Flags().StringVar(&subpath, "subpath", "", "Deploy WANIX to specified subpath")
	cmd.Flags().BoolVar(&enableAuth, "auth", false, "Enable authentication module")
	cmd.Flags().BoolVar(&requireAuth, "require-auth", false, "Require authentication")
	return cmd
}

var deployFiles = []string{
	"auth/api.js",
	"auth/auth0-9.23.3.min.js",
	"auth/auth0-spa-2.0.min.js",
	"auth/index.html",
	"index.html",
	"loading.gif",
	"wanix-bootloader.js",
	"wanix-kernel.gz",
	"wanix-initfs.gz",
}

var pagesIPs = []string{
	"185.199.108.153",
	"185.199.109.153",
	"185.199.110.153",
	"185.199.111.153",
}

var (
	oauthClientID     string
	oauthClientSecret string
)

func deploy(domain, subpath string, enableAuth, requireAuth bool) {

	ctx := context.Background()
	theme := huh.ThemeBase16()

	//accessibleMode := os.Getenv("ACCESSIBLE") != ""
	//form.WithAccessible(accessibleMode)

	if domain == "" {
		fatal(huh.NewInput().
			Title("Enter the domain to use for your auth capable GitHub Pages site:").
			Value(&domain).
			Validate(func(s string) error {
				if s == "" {
					return fmt.Errorf("Domain can't be empty")
				}
				return nil
			}).
			WithTheme(theme).
			Run())
	}

	var domainErr error
	fatal(spinner.New().
		Title(fmt.Sprintf("Checking DNS for domain '%s' ...", domain)).
		Action(func() {
			ips, err := net.LookupIP(domain)
			if err != nil {
				var dnsErr *net.DNSError
				if errors.As(err, &dnsErr) {
					domainErr = fmt.Errorf("Domain '%s' is not resolving to any IP.", domain)
					return
				} else {
					fatal(err)
				}
			}

			for _, ip := range ips {
				if !contains(pagesIPs, ip.String()) {
					domainErr = fmt.Errorf("Domain '%s' IPs not pointing to GitHub Pages.", domain)
					return
				}
			}
		}).
		Run())

	if domainErr != nil {
		text := lipgloss.NewStyle().Foreground(lipgloss.Color("#ffff00")).SetString(domainErr.Error())
		fmt.Println(text)

		fmt.Println()
		fmt.Println("Make sure to configure A records to the GitHub Pages IPs:")
		for _, ip := range pagesIPs {
			fmt.Println(" -", bright(ip))
		}
		fmt.Println()
		fmt.Println("It may take a few minutes for DNS changes to propagate.")
		os.Exit(1)
	}

	statusf("Domain '%s' is properly pointing to GitHub Pages.", domain)

	var tenantAuth auth0flow.Result
	if enableAuth {
		var hasAuth0 bool
		huh.NewConfirm().
			Title("Do you have an Auth0 account?").
			Value(&hasAuth0).
			Run()

		if !hasAuth0 {
			fmt.Println()
			fmt.Println("Get a free Auth0 account:")
			fmt.Println(bright("https://auth0.com/signup"))
			fmt.Println()
			os.Exit(1)
		}

		state, err := auth0flow.GetDeviceCode(ctx, http.DefaultClient, nil)
		if err != nil {
			log.Fatal("failed to get the device code:", err)
		}

		fmt.Printf("\nLogin to your Auth0 account with this URL ... \n\n%s\n\nBe sure to login with a tenant that we can clear and configure from scratch.\n", bright(state.VerificationURI))

		fatal(spinner.New().
			Title("").
			Action(func() {
				tenantAuth, err = auth0flow.WaitUntilUserLogsIn(ctx, http.DefaultClient, state)
				if err != nil {
					log.Fatal("failed to get the device code:", err)
				}
			}).
			Run())

		if tenantAuth.Tenant == "" {
			os.Exit(0)
		}

		fmt.Print("\033[A\033[K")
		fmt.Print("\033[A\033[K")
		fmt.Print("\033[A\033[K")
		fmt.Print("\033[A\033[K")
		fmt.Print("\033[A\033[K")
		fmt.Print("\033[A\033[K")

		statusf("Logged into Auth0 with tenant '%s' to be configured.", tenantAuth.Tenant)
	}

	var hasGithub bool
	huh.NewConfirm().
		Title("Do you have a GitHub account?").
		Value(&hasGithub).
		Run()

	if !hasGithub {
		fmt.Println()
		fmt.Println("Get a free GitHub account:")
		fmt.Println(bright("https://github.com/signup"))
		fmt.Println()
		os.Exit(1)
	}

	clientID := "b5faa9cd34a4fa21d844" // todo: update for wanix cli?
	ghCode, err := device.RequestCode(http.DefaultClient, "https://github.com/login/device/code", clientID, []string{"repo"})
	if err != nil {
		log.Fatal("req code:", err)
	}

	fmt.Printf("\nLogin to your GitHub account with this URL and code ...\n\n%s\nEnter code: %s\n\n", bright(ghCode.VerificationURI), bright(ghCode.UserCode))

	var ghAuth *api.AccessToken
	var gh *github.Client
	var user *github.User
	fatal(spinner.New().
		Title("").
		Action(func() {
			ghAuth, err = device.Wait(ctx, http.DefaultClient, "https://github.com/login/oauth/access_token", device.WaitOptions{
				ClientID:   clientID,
				DeviceCode: ghCode,
			})
			if err != nil {
				log.Fatal("device wait:", err)
			}

			gh = github.NewClient(nil).WithAuthToken(ghAuth.Token)
			user, _, err = gh.Users.Get(ctx, "")
			if err != nil {
				log.Fatal("user get:", err)
			}
		}).
		Run())

	fmt.Print("\033[A\033[K")
	fmt.Print("\033[A\033[K")
	fmt.Print("\033[A\033[K")
	fmt.Print("\033[A\033[K")
	fmt.Print("\033[A\033[K")
	fmt.Print("\033[A\033[K")

	statusf("Logged into GitHub as '%s'.", user.GetLogin())

	var domainClient *management.Client
	if enableAuth {
		fmt.Println()
		fmt.Println("Register a GitHub OAuth application to be used for login on your site:")
		fmt.Println(bright("https://github.com/settings/applications/new"))
		fmt.Println()
		printTable([][]string{
			{"Application name:", domain},
			{"Homepage URL:", fmt.Sprintf("https://%s", domain)},
			{"Authorization callback URL:", fmt.Sprintf("https://%s/login/callback\n", tenantAuth.Domain)},
		})

		huh.NewNote().
			Description("When finished, leave page open and press any key to continue").
			Run()

		fmt.Print("\033[A\033[K")
		fmt.Print("\033[A\033[K")
		fmt.Print("\033[A\033[K")
		fmt.Print("\033[A\033[K")
		fmt.Print("\033[A\033[K")
		fmt.Print("\033[A\033[K")
		fmt.Print("\033[A\033[K")
		fmt.Print("\033[A\033[K")

		huh.NewInput().
			Title(fmt.Sprintf("Enter the OAuth application %s:", bright("Client ID"))).
			Value(&oauthClientID).
			Run()

		huh.NewInput().
			Title(fmt.Sprintf("Generate and enter %s:", bright("Client Secret"))).
			Password(true).
			Value(&oauthClientSecret).
			Run()

		var confirmAuth0 bool
		fatal(huh.NewConfirm().
			Title(fmt.Sprintf("We are about to reset and configure the Auth0 tenant '%s'.\nAre you sure you want to continue?", tenantAuth.Tenant)).
			Affirmative("Yes").
			Negative("No").
			Value(&confirmAuth0).
			Run())

		if !confirmAuth0 {
			os.Exit(0)
		}

		api, err := management.New(
			tenantAuth.Domain,
			management.WithStaticToken(tenantAuth.AccessToken),
		)
		if err != nil {
			log.Fatalf("failed to initialize the auth0 management API client: %+v", err)
		}

		fatal(spinner.New().
			Title("Deleting existing clients...").
			Action(func() {
				cl, err := api.Client.List(ctx)
				if err != nil {
					log.Fatal("list clients:", err)
				}
				for _, c := range cl.Clients {
					if c.GetName() == "All Applications" {
						continue
					}
					if err := api.Client.Delete(ctx, c.GetClientID()); err != nil {
						log.Fatal("delete client:", err)
					}
				}
			}).
			Run())

		var internalClient *management.Client
		fatal(spinner.New().
			Title("Creating internal client used by on-login action...").
			Action(func() {
				internalClient = &management.Client{
					Name:        auth0.String("internal"),
					Description: auth0.String("used by on-login action"),
					AppType:     auth0.String("non_interactive"),
				}
				err = api.Client.Create(ctx, internalClient)
				if err != nil {
					log.Fatal("create internal client:", err)
				}
			}).
			Run())

		fatal(spinner.New().
			Title("Creating main client...").
			Action(func() {
				authURLs := []string{fmt.Sprintf("https://%s/auth/", domain)}
				origins := []string{fmt.Sprintf("https://%s", domain)}
				domainClient = &management.Client{
					Name:                    auth0.String(domain),
					Description:             auth0.String(domain),
					AppType:                 auth0.String("spa"),
					Callbacks:               &authURLs,
					AllowedLogoutURLs:       &authURLs,
					AllowedOrigins:          &origins,
					WebOrigins:              &origins,
					TokenEndpointAuthMethod: auth0.String("none"),
				}
				err = api.Client.Create(ctx, domainClient)
				if err != nil {
					log.Fatal("create client:", err)
				}
			}).
			Run())

		fatal(spinner.New().
			Title("Clearing connections...").
			Action(func() {
				conns, err := api.Connection.List(ctx)
				if err != nil {
					log.Fatal("conn list:", err)
				}
				for _, conn := range conns.Connections {
					if err := api.Connection.Delete(ctx, conn.GetID()); err != nil {
						log.Fatal("conn delete:", err)
					}
				}
			}).
			Run())

		fatal(spinner.New().
			Title("Waiting until cleared...").
			Action(func() {
				for {
					<-time.After(1 * time.Second)
					conns, err := api.Connection.List(ctx)
					if err != nil {
						log.Fatal("conn list:", err)
					}
					if conns.Total == 0 {
						break
					}
				}
			}).
			Run())

		fatal(spinner.New().
			Title("Setting up GitHub connection...").
			Action(func() {
				enabledClients := []string{domainClient.GetClientID()}
				err = api.Connection.Create(ctx, &management.Connection{
					Strategy:       auth0.String("github"),
					Name:           auth0.String("github"),
					EnabledClients: &enabledClients,
					Options: map[string]any{
						"follow":           false,
						"profile":          true,
						"read_org":         false,
						"admin_org":        false,
						"read_user":        false,
						"write_org":        false,
						"delete_repo":      false,
						"public_repo":      false,
						"repo_status":      false,
						"notifications":    false,
						"read_repo_hook":   false,
						"admin_repo_hook":  false,
						"read_public_key":  false,
						"repo_deployment":  false,
						"write_repo_hook":  false,
						"admin_public_key": false,
						"write_public_key": false,
						"gist":             false,
						"repo":             true,
						"email":            false,
						"scope":            []string{"repo"},
						"client_id":        oauthClientID,
						"client_secret":    oauthClientSecret,
					},
				})
				if err != nil {
					log.Fatal("conn create:", err)
				}
			}).
			Run())

		fatal(spinner.New().
			Title("Clearing post-login bindings...").
			Action(func() {
				err = api.Action.UpdateBindings(ctx, "post-login", []*management.ActionBinding{})
				if err != nil {
					log.Fatal("update binding:", err)
				}
			}).
			Run())

		fatal(spinner.New().
			Title("Clearing actions...").
			Action(func() {
				al, err := api.Action.List(ctx)
				if err != nil {
					log.Fatal("list actions:", err)
				}
				for _, a := range al.Actions {
					if auth0.StringValue(a.Name) == "on-login" {
						err = api.Action.Delete(ctx, a.GetID())
						if err != nil {
							log.Fatal("delete action:", err)
						}
					}
				}
			}).
			Run())

		var loginAction *management.Action
		fatal(spinner.New().
			Title("Creating on-login action...").
			Action(func() {
				tl, err := api.Action.Triggers(ctx)
				if err != nil {
					log.Fatal("list triggers:", err)
				}
				var trigger management.ActionTrigger
				for _, t := range tl.Triggers {
					if t.GetID() == "post-login" && t.GetStatus() == "CURRENT" {
						trigger = *t
						break
					}
				}
				if trigger.ID == nil {
					log.Fatal("unable to find post-login")
				}

				deps := []management.ActionDependency{{
					Name:    auth0.String("auth0"),
					Version: auth0.String("latest"),
				}}
				secrets := []management.ActionSecret{
					{Name: auth0.String("domain"), Value: auth0.String(tenantAuth.Domain)},
					{Name: auth0.String("admin"), Value: auth0.String(user.GetLogin())},
					{Name: auth0.String("clientId"), Value: auth0.String(internalClient.GetClientID())},
					{Name: auth0.String("clientSecret"), Value: auth0.String(internalClient.GetClientSecret())},
				}
				code, err := fs.ReadFile(boot.Dir, "site/auth/on-login.js")
				if err != nil {
					log.Fatal("readfile:", err)
				}
				loginAction = &management.Action{
					Name:              auth0.String("on-login"),
					SupportedTriggers: []management.ActionTrigger{trigger},
					Dependencies:      &deps,
					Secrets:           &secrets,
					Code:              auth0.String(string(code)),
					Runtime:           auth0.String("node18"),
				}
				err = api.Action.Create(ctx, loginAction)
				if err != nil {
					log.Fatal("create action:", err)
				}
			}).
			Run())

		fatal(spinner.New().
			Title("Waiting for on-login action to exist...").
			Action(func() {
				for {
					al, err := api.Action.List(ctx)
					if err != nil {
						log.Fatal("list actions:", err)
					}
					found := false
					for _, a := range al.Actions {
						if a.GetID() == loginAction.GetID() && a.GetStatus() == "built" {
							found = true
							break
						}
					}
					if found {
						break
					}
					<-time.After(1 * time.Second)
				}
			}).
			Run())

		fatal(spinner.New().
			Title("Deploying on-login action...").
			Action(func() {
				_, err = api.Action.Deploy(ctx, loginAction.GetID())
				if err != nil {
					log.Fatal("deploy action:", err)
				}
			}).
			Run())

		fatal(spinner.New().
			Title("Waiting for deployment...").
			Action(func() {
				for {
					<-time.After(1 * time.Second)
					vl, err := api.Action.Versions(ctx, loginAction.GetID())
					if err != nil {
						log.Fatal("list versions:", err)
					}
					if vl.Total >= 1 {
						break
					}
				}
			}).
			Run())

		fatal(spinner.New().
			Title("Creating post-login binding...").
			Action(func() {
				binding := management.ActionBinding{
					DisplayName: auth0.String("on-login"),
					Ref: &management.ActionBindingReference{
						Type:  auth0.String("action_name"),
						Value: auth0.String("on-login"),
					},
				}
				err = api.Action.UpdateBindings(ctx, "post-login", []*management.ActionBinding{&binding})
				if err != nil {
					log.Fatal("update binding:", err)
				}
			}).
			Run())

		fatal(spinner.New().
			Title("Deleting client grants...").
			Action(func() {
				gl, err := api.ClientGrant.List(ctx)
				if err != nil {
					log.Fatal("list grants:", err)
				}
				for _, g := range gl.ClientGrants {
					err = api.ClientGrant.Delete(ctx, g.GetID())
					if err != nil {
						log.Fatal("delete grant:", err)
					}
				}
			}).
			Run())

		fatal(spinner.New().
			Title("Creating internal client grant...").
			Action(func() {
				scope := []string{"read:user_idp_tokens", "read:users"}
				err = api.ClientGrant.Create(ctx, &management.ClientGrant{
					Scope:    &scope,
					Audience: auth0.String(fmt.Sprintf("https://%s/api/v2/", tenantAuth.Domain)),
					ClientID: auth0.String(internalClient.GetClientID()),
				})
				if err != nil {
					log.Fatal("create grant:", err)
				}
			}).
			Run())

		statusf("Auth0 tenant '%s' has been configured.", tenantAuth.Tenant)
	}

	repoName := domain
	username := user.GetLogin()
	branch := "gh-pages"
	pathname := "/"

	fatal(huh.NewInput().
		Title("Enter a repository name to be used for the site:").
		Value(&repoName).
		Run())

	fatal(huh.NewInput().
		Title("Enter a branch name for GitHub Pages to use:").
		Value(&branch).
		Run())

	var resp *github.Response
	var repo *github.Repository
	fatal(spinner.New().
		Title("Checking for repository...").
		Action(func() {
			repo, resp, err = gh.Repositories.Get(ctx, username, repoName)
			if err != nil && resp.StatusCode != 404 {
				log.Fatal("get repo:", err)
			}
		}).
		Run())

	if resp.StatusCode == 404 {
		fatal(spinner.New().
			Title("Creating repository...").
			Action(func() {
				repo, _, err = gh.Repositories.Create(ctx, "", &github.Repository{
					Name: github.String(repoName),
				})
				if err != nil {
					log.Fatal("create repo:", err)
				}
			}).
			Run())
	} else {
		var confirmExistingRepo bool
		fatal(huh.NewConfirm().
			Title(fmt.Sprintf("Repository '%s/%s' already exists.\nContinue to push files to %s?", username, repoName, branch)).
			Affirmative("Yes").
			Negative("No").
			Value(&confirmExistingRepo).
			Run())

		if !confirmExistingRepo {
			os.Exit(0)
		}
	}

	_, resp, err = gh.Git.GetRef(ctx, username, repoName, "refs/heads/"+branch)
	if resp.StatusCode == 404 {
		ref, _, err := gh.Git.GetRef(ctx, username, repoName, "refs/heads/"+repo.GetDefaultBranch())
		if err != nil {
			log.Fatal("get ref:", err)
		}
		_, _, err = gh.Git.CreateRef(ctx, username, repoName, &github.Reference{
			Ref: github.String("refs/heads/" + branch),
			Object: &github.GitObject{
				SHA: ref.Object.SHA,
			},
		})
		if err != nil {
			log.Fatal("create ref:", err)
		}
	}

	uploadTitle := "Uploading WANIX site..."
	if enableAuth {
		uploadTitle = "Uploading WANIX site and auth module..."
	}
	fatal(spinner.New().
		Title(uploadTitle).
		Action(func() {
			bl, err := buildBootloader()
			fatal(err)
			kernel, err := fs.ReadFile(boot.Dir, "kernel.gz")
			fatal(err)
			initfs, err := fs.ReadFile(boot.Dir, "initfs.gz")
			fatal(err)
			bootfiles := map[string][]byte{
				"wanix-kernel.gz":     kernel,
				"wanix-initfs.gz":     initfs,
				"wanix-bootloader.js": bl,
			}

			for _, filepath := range deployFiles {
				if !enableAuth && strings.HasPrefix(filepath, "auth/") {
					continue
				}
				targetpath := filepath
				if subpath != "" && !strings.HasPrefix(filepath, "auth/") {
					targetpath = path.Join(strings.TrimLeft(subpath, "/"), filepath)
				}
				var sha *string
				f, _, _, err := gh.Repositories.GetContents(ctx, username, repoName, targetpath, &github.RepositoryContentGetOptions{
					Ref: branch,
				})
				if f != nil {
					sha = f.SHA
				}
				var data []byte
				if d, ok := bootfiles[filepath]; ok {
					data = d
				} else {
					data, err = fs.ReadFile(boot.Dir, path.Join("site", filepath))
					if err != nil {
						panic(err)
					}
				}
				if filepath == "index.html" {
					t := template.Must(template.New("index.html").Parse(string(data)))
					var buf bytes.Buffer
					if err := t.Execute(&buf, map[string]any{
						"RequireAuth": requireAuth,
						"Username":    username,
						"RepoName":    repoName,
					}); err != nil {
						panic(err)
					}
					data = buf.Bytes()
				}
				if filepath == "auth/index.html" {
					data = []byte(fmt.Sprintf(string(data), tenantAuth.Domain, domainClient.GetClientID()))
				}
				attempts := 0
				for {
					attempts += 1
					_, _, err = gh.Repositories.UpdateFile(ctx, username, repoName, targetpath, &github.RepositoryContentFileOptions{
						Message: github.String("wanix deploy"),
						Branch:  github.String(branch),
						Content: data,
						SHA:     sha,
					})
					if err == nil || attempts == 3 {
						break
					}
				}
				if err != nil {
					log.Fatal("commit file:", err)
				}
			}
		}).
		Run())

	fatal(spinner.New().
		Title("Checking pages...").
		Action(func() {
			_, resp, err = gh.Repositories.GetPagesInfo(ctx, username, repoName)
			if err != nil && resp.StatusCode != 404 {
				log.Fatal(err)
			}
		}).
		Run())

	if resp.StatusCode == 404 {
		fatal(spinner.New().
			Title("Creating pages...").
			Action(func() {
				_, _, err := gh.Repositories.EnablePages(ctx, username, repoName, &github.Pages{
					Source: &github.PagesSource{
						Branch: github.String(branch),
						Path:   github.String(pathname),
					},
				})
				if err != nil {
					log.Fatal(err)
				}
			}).
			Run())
	}

	fatal(spinner.New().
		Title("Setting CNAME...").
		Action(func() {
			_, err = gh.Repositories.UpdatePages(ctx, username, repoName, &github.PagesUpdate{
				CNAME: github.String(domain),
				Source: &github.PagesSource{
					Branch: github.String(branch),
					Path:   github.String(pathname),
				},
			})
			if err != nil {
				log.Fatal(err)
			}
		}).
		Run())

	fatal(spinner.New().
		Title("Checking cert status...").
		Action(func() {
			for {
				pages, _, err := gh.Repositories.GetPagesInfo(ctx, username, repoName)
				if err != nil {
					log.Fatal(err)
				}
				if pages.HTTPSCertificate != nil && (*pages.HTTPSCertificate.State) == "approved" {
					break
				}
				<-time.After(2 * time.Second)
			}
		}).
		Run())

	fatal(spinner.New().
		Title("Setting enforce HTTPS...").
		Action(func() {
			_, err = gh.Repositories.UpdatePages(ctx, username, repoName, &github.PagesUpdate{
				HTTPSEnforced: github.Bool(true),
				CNAME:         github.String(domain),
				Source: &github.PagesSource{
					Branch: github.String(branch),
					Path:   github.String(pathname),
				},
			})
			if err != nil {
				log.Fatal(err)
			}
		}).
		Run())

	statusf("GitHub repository '%s' has been configured for GitHub Pages.", repoName)

	fatal(spinner.New().
		Title("Waiting while site is being deployed...").
		Action(func() {
			for {
				pages, _, err := gh.Repositories.GetPagesInfo(ctx, username, repoName)
				if err != nil {
					log.Fatal(err)
				}
				if (*pages.Status) == "built" {
					return
				}
				<-time.After(2 * time.Second)
			}
		}).
		Run())

	fmt.Println("\r")
	if subpath != "" {
		subpath = "/" + strings.TrimPrefix(subpath, "/")
	}
	if enableAuth {
		printTable([][]string{
			{"GitHub repository:", fmt.Sprintf("https://github.com/%s/%s", username, repoName)},
			{"Auth0 dashboard:", fmt.Sprintf("https://manage.auth0.com/dashboard/us/%s/", tenantAuth.Tenant)}, // TODO: fix region
			{"", ""},
			{"WANIX deployed:", bright(fmt.Sprintf("https://%s%s", domain, subpath))},
		})
	} else {
		printTable([][]string{
			{"GitHub repository:", fmt.Sprintf("https://github.com/%s/%s", username, repoName)},
			{"", ""},
			{"WANIX deployed:", bright(fmt.Sprintf("https://%s%s", domain, subpath))},
		})
	}
	fmt.Println()
	fmt.Println("🎉 Congrats! Your WANIX site is set up and live!")

	// b, err := json.MarshalIndent(map[string]any{
	// 	"domain": domain,
	// }, "", "  ")
	// fatal(err)
	// configfile := fmt.Sprintf("%s.json", strings.ReplaceAll(domain, ".", "-"))
	// fatal(os.WriteFile(configfile))
	// fmt.Printf("Wrote configuration to %s", configfile)
}

func contains(slice []string, str string) bool {
	for _, item := range slice {
		if item == str {
			return true
		}
	}
	return false
}

func statusf(format string, args ...any) {
	prefix := fmt.Sprintf("\r%s ", lipgloss.NewStyle().Foreground(lipgloss.Color("#00ff00")).Render("✓"))
	fmt.Println(prefix + huh.ThemeBase16().Focused.Title.Render(fmt.Sprintf(format, args...)))
}

func bright(text string) string {
	return lipgloss.NewStyle().Foreground(lipgloss.Color("#ffffff")).Render(text)
}

func printTable(data [][]string) {
	table := tablewriter.NewWriter(os.Stdout)
	table.SetAutoWrapText(false)
	table.SetAutoFormatHeaders(true)
	table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
	table.SetAlignment(tablewriter.ALIGN_LEFT)
	table.SetCenterSeparator("")
	table.SetColumnSeparator("")
	table.SetRowSeparator("")
	table.SetHeaderLine(false)
	table.SetBorder(false)
	table.SetTablePadding("\t")
	table.SetNoWhiteSpace(true)
	table.AppendBulk(data)
	table.Render()
}
