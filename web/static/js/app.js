document.addEventListener("click", async (event) => {
    const button = event.target.closest("[data-copy]");
    if (!button) return;

    const value = button.dataset.copy;
    const original = button.innerHTML;

    try {
        await navigator.clipboard.writeText(value);
        button.textContent = "Copied";
        button.classList.add("copied");
    } catch {
        const field = document.createElement("textarea");
        field.value = value;
        field.setAttribute("readonly", "");
        field.style.position = "absolute";
        field.style.left = "-9999px";
        document.body.appendChild(field);
        field.select();
        const copied = document.execCommand("copy");
        field.remove();

        button.textContent = copied ? "Copied" : "Copy failed";
    }

    window.setTimeout(() => {
        button.innerHTML = original;
        button.classList.remove("copied");
    }, 1400);
});
