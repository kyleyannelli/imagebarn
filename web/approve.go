package web

import (
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"sort"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/utils"
	"kmfg.dev/imagebarn/v1/filestore"
	"kmfg.dev/imagebarn/v1/helpme"
)

type ViewApprovedUser struct {
	Email      string
	IsApproved bool
}

func (vau *ViewApprovedUser) AdminUserEmail() string {
	return AdminUserEmail
}

const PARTIALS_APPROVE_VIEW = BASE_PARTIAL + "/approve"

func RegisterApprover(barnage *BarnageWeb) {
	approveRouter := barnage.fiber.Group("/approve")
	approveRouter.Use(adminCheckMiddleware)
	approveRouter.Get("", showAll)
	approveRouter.Post("", showAllSearch)
	approveRouter.Put("/:email", approve)
	barnage.fiber.Group("/disapprove").Use(adminCheckMiddleware).Put("/:email", disapprove)
}

func showAllSearch(c *fiber.Ctx) error {
	searchQuery := c.FormValue("search", "")
	if searchQuery == "" {
		return showAll(c)
	}

	copiedMap := barnage.fs.ApprovedUsers().CopyOfUsersMap()
	var emailScores []helpme.Alike

	minLikeness := float32(0.1)

	for email := range copiedMap {
		res := CompareTwoStringsOptimized(email, searchQuery)
		if res >= minLikeness {
			emailScores = append(emailScores, helpme.Alike{String: email, Score: res})
		}
	}

	// maybe use min heap instead?
	sort.Slice(emailScores, func(i, j int) bool {
		return emailScores[i].Score > emailScores[j].Score
	})

	N := 3
	if len(emailScores) < N {
		N = len(emailScores)
	}

	matchedSlice := make([]*ViewApprovedUser, N)
	for i := 0; i < N; i++ {
		matchedSlice[i] = &ViewApprovedUser{
			Email:      emailScores[i].String,
			IsApproved: copiedMap[emailScores[i].String],
		}
	}

	return c.Render(PARTIALS_APPROVE_VIEW, fiber.Map{
		"ApprovedUsersSlice": matchedSlice,
	})
}

func showAll(c *fiber.Ctx) error {
	page := c.Query("page", "")
	pageInt, err := strconv.Atoi(page)
	if err != nil || pageInt < 0 {
		pageInt = 0
	} else {
		pageInt--
	}

	copiedMap := barnage.fs.ApprovedUsers().CopyOfUsersMap()

	approvedSlice := make([]ViewApprovedUser, 0, len(copiedMap))
	for email, isApproved := range copiedMap {
		approvedSlice = append(approvedSlice, ViewApprovedUser{Email: email, IsApproved: isApproved})
	}

	sort.Slice(approvedSlice, func(i, j int) bool {
		return approvedSlice[i].Email < approvedSlice[j].Email
	})

	offset := 3
	var paginatedSlice []ViewApprovedUser

	if pageInt == 0 {
		if len(approvedSlice) > offset {
			paginatedSlice = approvedSlice[:offset]
		} else {
			paginatedSlice = approvedSlice
		}
	} else {
		startIdx := offset * pageInt
		endIdx := startIdx + offset

		if startIdx >= len(approvedSlice) {
			paginatedSlice = []ViewApprovedUser{}
		} else if endIdx > len(approvedSlice) {
			paginatedSlice = approvedSlice[startIdx:]
		} else {
			paginatedSlice = approvedSlice[startIdx:endIdx]
		}
	}

	pageInt++
	totalItems := len(approvedSlice)
	totalPages := (totalItems + offset - 1) / offset
	offset = 1

	desiredButtons := 1 + 1 + 1 + (2 * offset)
	if desiredButtons > totalPages {
		desiredButtons = totalPages
	}

	availablePagesSet := make(map[int]struct{})

	availablePagesSet[1] = struct{}{}
	availablePagesSet[pageInt] = struct{}{}
	availablePagesSet[totalPages] = struct{}{}

	numPagesAdded := len(availablePagesSet)

	left := pageInt - 1
	right := pageInt + 1

	for numPagesAdded < desiredButtons && (left >= 1 || right <= totalPages) {
		if left >= 1 && numPagesAdded < desiredButtons {
			if _, exists := availablePagesSet[left]; !exists {
				availablePagesSet[left] = struct{}{}
				numPagesAdded++
			}
			left--
		}

		if right <= totalPages && numPagesAdded < desiredButtons {
			if _, exists := availablePagesSet[right]; !exists {
				availablePagesSet[right] = struct{}{}
				numPagesAdded++
			}
			right++
		}
	}

	availablePages := make([]int, 0, len(availablePagesSet))
	for page := range availablePagesSet {
		availablePages = append(availablePages, page)
	}

	sort.Ints(availablePages)

	return c.Render(PARTIALS_APPROVE_VIEW, fiber.Map{
		"ApprovedUsersSlice": paginatedSlice,
		"AdminUserEmail":     AdminUserEmail,
		"CurrentPage":        pageInt,
		"AvailablePages":     availablePages,
	})
}

func approve(c *fiber.Ctx) error {
	emailToApprove, err := obtainEmail(c)
	if err != nil {
		return err
	}
	if emailToApprove == AdminUserEmail {
		return c.SendStatus(400)
	}
	slog.Info(fmt.Sprintf("Approving %v", emailToApprove))
	barnage.fs.ApprovedUsers().Approve(emailToApprove)
	err = os.MkdirAll(fmt.Sprintf("./images/%v", filestore.Encode(emailToApprove)), 0700)
	if err != nil {
		slog.Debug(fmt.Sprintf("Couldn't make dir for new approved user: %v", err))
	}
	return showAllSearch(c)
}

func disapprove(c *fiber.Ctx) error {
	emailToDisapprove, err := obtainEmail(c)
	if err != nil {
		return err
	}
	if emailToDisapprove == AdminUserEmail {
		return c.SendStatus(400)
	}
	slog.Info(fmt.Sprintf("Disapproving %v", emailToDisapprove))
	barnage.fs.ApprovedUsers().Disapprove(emailToDisapprove)
	if err := filestore.DeleteAll(emailToDisapprove); err != nil {
		slog.Warn(fmt.Sprintf("Failed to remove dir for %v: %v", emailToDisapprove, err))
	} else {
		slog.Info(fmt.Sprintf("Removed %v images", emailToDisapprove))
	}
	return showAllSearch(c)
}

func adminCheckMiddleware(c *fiber.Ctx) error {
	email, valid := getEmailFromJWT(c.Cookies("jwt", ""))
	if !valid {
		return fmt.Errorf("Invalid JWT!")
	}
	if email != AdminUserEmail {
		return c.SendStatus(403)
	}
	return c.Next()
}

func obtainEmail(c *fiber.Ctx) (string, error) {
	emailEsc := utils.CopyString(c.Params("email", ""))
	if emailEsc == "" {
		return "", fmt.Errorf("Couldn't find email in path!")
	}
	// need to use path over query because + can be used in an email and wont translate to " "
	emailUnesc, err := url.PathUnescape(emailEsc)
	if err != nil {
		return "", fmt.Errorf("Failed to unescape %v: %v", emailEsc, err)
	} else {
		return emailUnesc, nil
	}
}
