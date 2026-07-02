package adapter

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsHDR_PQ(t *testing.T) {
	assert.True(t, IsHDR("smpte2084"))
}

func TestIsHDR_HLG(t *testing.T) {
	assert.True(t, IsHDR("arib-std-b67"))
}

func TestIsHDR_SDR(t *testing.T) {
	assert.False(t, IsHDR("bt709"))
}

func TestIsHDR_Empty(t *testing.T) {
	assert.False(t, IsHDR(""))
}

func TestIsPortrait_Landscape(t *testing.T) {
	assert.False(t, IsPortrait(1920, 1080))
}

func TestIsPortrait_Portrait(t *testing.T) {
	assert.True(t, IsPortrait(1080, 1920))
}

func TestIsPortrait_Square(t *testing.T) {
	assert.False(t, IsPortrait(1080, 1080))
}

func TestBuildVFChain_SDR_Landscape(t *testing.T) {
	chain := BuildVFChain("bt709", false, 1920, 1080, nil)
	// SDR 无 tonemap，只有 scale
	assert.False(t, strings.Contains(chain, "tonemap"))
	assert.True(t, strings.Contains(chain, "scale=1920:-2"))
}

func TestBuildVFChain_HDR_Landscape(t *testing.T) {
	chain := BuildVFChain("smpte2084", false, 1920, 1080, nil)
	// HDR 应含 tonemap 链 + scale
	assert.True(t, strings.Contains(chain, "tonemap=hable"))
	assert.True(t, strings.Contains(chain, "scale=1920:-2"))
}

func TestBuildVFChain_Portrait(t *testing.T) {
	chain := BuildVFChain("bt709", true, 1920, 1080, nil)
	// 竖屏按高度缩放
	assert.True(t, strings.Contains(chain, "scale=-2:1080"))
}

func TestBuildVFChain_WithExtraFilters(t *testing.T) {
	chain := BuildVFChain("bt709", false, 1920, 1080, []string{"eq=contrast=1.03"})
	assert.True(t, strings.Contains(chain, "eq=contrast=1.03"))
}

func TestBuildAudioFadeChain_Normal(t *testing.T) {
	chain := BuildAudioFadeChain(5.0)
	assert.True(t, strings.Contains(chain, "afade=t=in:st=0:d=0.030"))
	assert.True(t, strings.Contains(chain, "afade=t=out:st=4.970:d=0.030"))
}

func TestBuildAudioFadeChain_VeryShort(t *testing.T) {
	// 短于 30ms 的段：fadeOutStart 钳为 0，不产生负值
	chain := BuildAudioFadeChain(0.02)
	assert.True(t, strings.Contains(chain, "afade=t=out:st=0.000:d=0.030"))
}
