package bot

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/etkecc/postmoogle/internal/bot/config"
	"github.com/etkecc/postmoogle/internal/email"
	"github.com/etkecc/postmoogle/internal/utils"
)

const (
	// maxImageSize limits the size of a single email image, in bytes
	maxImageSize = 5 * 1024 * 1024
	// maxEmailImages limits how many images of an email are shown inside the message
	maxEmailImages = 50
	// maxRemoteImages limits how many images linked in an email are downloaded
	maxRemoteImages = 20
	// imageWorkers is how many images are downloaded and uploaded in parallel
	imageWorkers = 4
	// imagesTimeout limits the total time spent on the images of an email
	imagesTimeout = 30 * time.Second
)

var errImageUnavailable = errors.New("image is not available")

// embedImages uploads the images of an email to the Matrix content repository, so they are shown inside the message
func (b *Bot) embedImages(ctx context.Context, eml *email.Email, cfg config.Room) {
	options := cfg.ContentOptions()
	if !options.HTML || (!options.InlineImages && !options.RemoteImages) {
		return
	}
	sources := imagesToLoad(eml, options)
	if len(sources) == 0 {
		return
	}

	ctx, cancel := context.WithTimeout(ctx, imagesTimeout)
	defer cancel()
	images := make([]*email.Image, len(sources))
	queue := make(chan int)
	var wg sync.WaitGroup
	for range min(imageWorkers, len(sources)) {
		wg.Go(func() {
			for i := range queue {
				images[i] = b.loadImage(ctx, eml, sources[i])
			}
		})
	}
	for i := range sources {
		queue <- i
	}
	close(queue)
	wg.Wait()

	for i, src := range sources {
		eml.SetImage(src, images[i])
	}
}

// imagesToLoad returns image sources allowed by the room options that were not processed yet, within limits
func imagesToLoad(eml *email.Email, options *email.ContentOptions) []string {
	var sources []string
	var total, remote int
	for _, src := range eml.ImageSources() {
		kind := email.ImageKind(src)
		allowed := options.InlineImages
		if kind == email.ImageRemote {
			remote++
			allowed = options.RemoteImages && remote <= maxRemoteImages
		}
		total++
		if !allowed || total > maxEmailImages || eml.HasImage(src) {
			continue
		}
		sources = append(sources, src)
	}
	return sources
}

// loadImage gets the image of the source and uploads it to the Matrix content repository; nil if not possible
func (b *Bot) loadImage(ctx context.Context, eml *email.Email, src string) *email.Image {
	file, err := b.imageFile(ctx, eml, src)
	if err != nil {
		b.log.Debug().Err(err).Str("src", shorten(src, 100)).Msg("cannot load email image")
		return nil
	}
	resp, err := b.lp.GetClient().UploadMedia(ctx, *file.Convert())
	if err != nil {
		b.log.Warn().Err(err).Str("file", file.Name).Msg("cannot upload email image")
		return nil
	}

	width, height := file.ImageSize()
	img := &email.Image{URI: string(resp.ContentURI.CUString()), Width: width, Height: height}
	if email.ImageKind(src) == email.ImageInline {
		img.File = file
	}
	return img
}

func (b *Bot) imageFile(ctx context.Context, eml *email.Email, src string) (*utils.File, error) {
	switch email.ImageKind(src) {
	case email.ImageInline:
		file := eml.InlineFile(src)
		if file == nil || !file.IsWebImage() || file.Length > maxImageSize {
			return nil, errImageUnavailable
		}
		return file, nil
	case email.ImageData:
		return utils.FileFromDataURI(src, maxImageSize)
	case email.ImageRemote:
		if strings.HasPrefix(src, "//") {
			src = "https:" + src
		}
		return b.images.Fetch(ctx, src)
	default:
		return nil, errImageUnavailable
	}
}

func shorten(text string, length int) string {
	if len(text) <= length {
		return text
	}
	return text[:length] + "..."
}
