env CGO_ENABLED=1 go test -race -tags oracle ./internal/testkit -run \^TestReviewSecondaryOfficialFixtures\$ -count=1 -v 
