<script>
  export let width = "300px";
  export let height = "25px";
  export let padding = "auto";
  export let radius = "4px";
</script>

<div
  class="loader"
  style={`width: ${width}; height: ${height}; padding: ${padding}; border-radius: ${radius}`}
  aria-hidden="true"
>
  <div class="effect"></div>
</div>

<style>
  .loader {
    overflow: hidden;
    /* padding: 25px 0; */
    box-sizing: border-box;
    position: relative;
    flex: 0 0 auto;
  }

  .effect {
    position: absolute;
    top: 0;
    left: 0;
    width: 100%;
    height: 100%;
    border-radius: inherit;
    background-color: white;
    opacity: 0.15;

    mask-image: linear-gradient(to right, transparent 0%, black 25%, black 75%, transparent 100%);
    -webkit-mask-image: linear-gradient(to right, transparent 0%, black 25%, black 75%, transparent 100%);
    mask-size: 200% 100%;
    -webkit-mask-size: 200% 100%;
    mask-repeat: repeat;
    -webkit-mask-repeat: repeat;
    mask-position: 50% top;
    -webkit-mask-position: 50% top;

    animation: wave 1s infinite;

    color: white;
  }

  @keyframes wave {
    0% {
      -webkit-mask-position: 50% top;
      mask-position: 50% top;
    }
    100% {
      -webkit-mask-position: -150% top;
      mask-position: -150% top;
    }
  }
</style>
