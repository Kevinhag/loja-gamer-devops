import "@testing-library/jest-dom/vitest";

// jsdom não implementa scroll
window.scrollTo = () => {};
