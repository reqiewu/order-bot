package tguser

import "testing"

func TestInitDataFromWebViewURL(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in, want string
	}{
		{
			"https://portals-market.com/#tgWebAppData=user%3D1%26hash%3Dabc&tgWebAppVersion=8.0",
			"user=1&hash=abc",
		},
		{
			"https://cdn.tgmrkt.io/?tgWebAppData=user%3D%7B%22id%22%3A1%7D%26hash%3Dx&tgWebAppVersion=7.10&tgWebAppPlatform=android",
			`user={"id":1}&hash=x`,
		},
		{
			"https://marketplace.tonnel.network/#tgWebAppData=auth_date%3D1%26hash%3Dz&tgWebAppThemeParams=%7B%7D",
			"auth_date=1&hash=z",
		},
		{
			"https://getgems.io/tg#tgWebAppData=user%3D%257B%2522id%2522%253A1%257D%26hash%3Dabc&tgWebAppVersion=9.1",
			`user={"id":1}&hash=abc`,
		},
	}
	for _, c := range cases {
		got, err := InitDataFromWebViewURL(c.in)
		if err != nil {
			t.Fatalf("InitDataFromWebViewURL(%q): %v", c.in, err)
		}
		if got != c.want {
			t.Fatalf("InitDataFromWebViewURL(%q)=\n%q\nwant\n%q", c.in, got, c.want)
		}
	}
}

func TestInitDataFromWebViewURL_missing(t *testing.T) {
	t.Parallel()
	if _, err := InitDataFromWebViewURL("https://example.com/"); err == nil {
		t.Fatal("expected error")
	}
}
